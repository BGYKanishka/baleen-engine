package network

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

// API Metadata struct (Invisible to CLI)
type PeerMeta struct {
	Source   string
	Status   string
	Arch     string
	LastSeen time.Time
}

// safely stores active nodes to prevent race conditions
type PeerRegistry struct {
	mu       sync.RWMutex
	nodes    map[string]string
	metadata map[string]*PeerMeta
	Log      io.Writer
}

func NewPeerRegistry() *PeerRegistry {
	return &PeerRegistry{
		nodes:    make(map[string]string),
		metadata: make(map[string]*PeerMeta),
	}
}

func (pr *PeerRegistry) AddPeer(name, address string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.nodes[name] = address

	// Track metadata for the API invisibly
	isNew := false
	if _, exists := pr.metadata[name]; !exists {
		pr.metadata[name] = &PeerMeta{Source: "mdns", Status: "reachable", Arch: "unknown", LastSeen: time.Now()}
		isNew = true
	} else {
		pr.metadata[name].LastSeen = time.Now()
		pr.metadata[name].Status = "reachable"
		if pr.metadata[name].Arch == "unknown" {
			isNew = true
		}
	}

	if isNew {
		go pr.detectAndUpdateArch(name, address)
	}
}

func (pr *PeerRegistry) detectAndUpdateArch(name, address string) {
	parts := strings.SplitN(address, ":", 2)
	if len(parts) == 2 {
		ip := parts[0]
		var port int
		fmt.Sscanf(parts[1], "%d", &port)
		if port > 0 {
			arch := DetectRemoteArch(ip, port)
			pr.mu.Lock()
			if meta, exists := pr.metadata[name]; exists {
				meta.Arch = arch
			}
			pr.mu.Unlock()
		}
	}
}

func (pr *PeerRegistry) GetAllPeers() map[string]string {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	copyMap := make(map[string]string)
	for k, v := range pr.nodes {
		copyMap[k] = v
	}
	return copyMap
}

// safely deletes a disconnected node from the registry
func (pr *PeerRegistry) RemovePeer(name string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	delete(pr.nodes, name)
	delete(pr.metadata, name) // Clean up API metadata
}

// constantly pings known peers to ensure they are still online
func (pr *PeerRegistry) StartHealthChecker() {
	for {
		time.Sleep(10 * time.Second)
		pr.mu.RLock()
		checkList := make(map[string]string)
		for name, addr := range pr.nodes {
			checkList[name] = addr
		}
		pr.mu.RUnlock()

		for name, addr := range checkList {
			conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
			if err != nil {
				pr.RemovePeer(name)
				if pr.Log != nil {
					fmt.Fprintf(pr.Log, "[Network] Peer disconnected: %s\n", name)
				}
			} else {
				conn.Close()
				// Update last seen for the API
				pr.mu.Lock()
				if meta, exists := pr.metadata[name]; exists {
					meta.LastSeen = time.Now()
				}
				pr.mu.Unlock()
			}
		}
	}
}

func StartBroadcaster(nodeName string, port int) {
	fmt.Printf("Broadcasting presence on local WiFi as '%s' (Port %d)...\n", nodeName, port)
	go func() {
		for {
			// Try to bind to the current WiFi network
			server, err := zeroconf.Register(nodeName, "_baleen._tcp", "local.", port, nil, nil)
			if err == nil {
				time.Sleep(30 * time.Second)
				server.Shutdown()
			} else {
				time.Sleep(5 * time.Second)
			}
		}
	}()
}

// acts as a Continuous Sweeper
func DiscoverPeers(currentNodeName string, registry *PeerRegistry) {
	go func() {
		for {
			// Create new resolver every cycle
			resolver, err := zeroconf.NewResolver(nil)
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}

			entries := make(chan *zeroconf.ServiceEntry)

			// Listen continuously
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

			go func(results <-chan *zeroconf.ServiceEntry) {
				for entry := range results {
					if entry.Instance != currentNodeName {

						// Ping all IPs and keep the real one
						var validAddress string
						for _, ipAddr := range entry.AddrIPv4 {
							testAddr := net.JoinHostPort(ipAddr.String(), fmt.Sprint(entry.Port))
							// fast 500ms ping
							conn, err := net.DialTimeout("tcp", testAddr, 500*time.Millisecond)
							if err == nil {
								conn.Close()
								validAddress = testAddr
								break
							}
						}

						if validAddress == "" {
							continue
						}

						prMap := registry.GetAllPeers()
						if _, exists := prMap[entry.Instance]; !exists {
							if registry.Log != nil {
								fmt.Fprintf(registry.Log, "[Discovery] Found Remote Peer: %s (IP: %s)\n", entry.Instance, validAddress)
							}
						}

						registry.AddPeer(entry.Instance, validAddress)
					}
				}
			}(entries)

			err = resolver.Browse(ctx, "_baleen._tcp", "local.", entries)
			if err != nil {
				fmt.Println("Failed to browse network:", err)
			}

			<-ctx.Done()
			cancel()
		}
	}()
}

// Additional methods for API integration
func (pr *PeerRegistry) AddCustomPeer(name, address, source string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.nodes[name] = address
	pr.metadata[name] = &PeerMeta{Source: source, Status: "reachable", Arch: "unknown", LastSeen: time.Now()}
	go pr.detectAndUpdateArch(name, address)
}

func LoadStaticPeers(pr *PeerRegistry) {
	envPeers := os.Getenv("BALEEN_PEERS")
	if envPeers == "" {
		return
	}
	ips := strings.Split(envPeers, ",")
	for i, ip := range ips {
		trimmed := strings.TrimSpace(ip)
		if trimmed != "" {
			name := fmt.Sprintf("static-peer-%d", i+1)
			pr.AddCustomPeer(name, trimmed, "static")
		}
	}
}

type APINode struct {
	Address  string
	Source   string
	Status   string
	Arch     string
	LastSeen time.Time
}

func (pr *PeerRegistry) GetDetailedPeers() map[string]*APINode {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	copyMap := make(map[string]*APINode)
	for name, addr := range pr.nodes {
		meta := pr.metadata[name]
		if meta == nil {
			meta = &PeerMeta{Source: "mdns", Status: "reachable", Arch: "unknown", LastSeen: time.Now()}
		}
		copyMap[name] = &APINode{
			Address:  addr,
			Source:   meta.Source,
			Status:   meta.Status,
			Arch:     meta.Arch,
			LastSeen: meta.LastSeen,
		}
	}
	return copyMap
}
