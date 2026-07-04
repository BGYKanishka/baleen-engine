package network

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

// API Metadata struct (Invisible to CLI)
type PeerMeta struct {
	Source      string
	Status      string
	Arch        string
	Fingerprint string
	LastSeen    time.Time
}

// safely stores active nodes to prevent race conditions
type PeerRegistry struct {
	mu       sync.RWMutex
	nodes    map[string]string
	metadata map[string]*PeerMeta
}

func NewPeerRegistry() *PeerRegistry {
	return &PeerRegistry{
		nodes:    make(map[string]string),
		metadata: make(map[string]*PeerMeta),
	}
}

func (pr *PeerRegistry) AddPeer(name, address, fingerprint string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.nodes[name] = address

	// Track metadata for the API invisibly
	isNew := false
	if _, exists := pr.metadata[name]; !exists {
		pr.metadata[name] = &PeerMeta{Source: "mdns", Status: "reachable", Arch: "unknown", Fingerprint: fingerprint, LastSeen: time.Now()}
		isNew = true
	} else {
		pr.metadata[name].LastSeen = time.Now()
		pr.metadata[name].Status = "reachable"
		pr.metadata[name].Fingerprint = fingerprint
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
func (pr *PeerRegistry) StartHealthChecker(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
		pr.mu.RLock()
		checkList := make(map[string]string)
		for name, addr := range pr.nodes {
			checkList[name] = addr
		}
		pr.mu.RUnlock()

		for name, addr := range checkList {
			conn, err := net.DialTimeout("tcp", addr, 2*time.Second)

			pr.mu.Lock()
			meta, exists := pr.metadata[name]
			if !exists {
				pr.mu.Unlock()
				continue
			}

			if err != nil {
				meta.Status = "unreachable"
				if meta.Source != "static" && time.Since(meta.LastSeen) > 60*time.Second {
					pr.mu.Unlock()
					pr.RemovePeer(name)
					slog.Info("peer disconnected", "peer", name)
					continue
				}
			} else {
				conn.Close()
				meta.Status = "reachable"
			}
			pr.mu.Unlock()
		}
	}
}

func StartBroadcaster(ctx context.Context, wg *sync.WaitGroup, nodeName string, port int, fingerprint string) {
	defer wg.Done()
	slog.Info("broadcasting presence on local WiFi", "nodeName", nodeName, "port", port)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		// Try to bind to the current WiFi network
		server, err := zeroconf.Register(nodeName, "_baleen._tcp", "local.", port, []string{"fp=" + fingerprint}, nil)
		if err == nil {
			select {
			case <-ctx.Done():
				server.Shutdown()
				return
			case <-time.After(30 * time.Second):
				server.Shutdown()
			}
		} else {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

// acts as a Continuous Sweeper
func DiscoverPeers(ctx context.Context, wg *sync.WaitGroup, currentNodeName string, registry *PeerRegistry) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Create new resolver every cycle
		resolver, err := zeroconf.NewResolver(nil)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
				continue
			}
		}

		entries := make(chan *zeroconf.ServiceEntry)

		// Listen continuously
		browseCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

		go func(results <-chan *zeroconf.ServiceEntry) {
			for entry := range results {
				if entry.Instance != currentNodeName {

					// Ping all IPs and keep the real one
					var validAddress string
					for _, ipAddr := range entry.AddrIPv4 {
						testAddr := net.JoinHostPort(ipAddr.String(), fmt.Sprint(entry.Port))
						// fast 1s ping
						conn, err := net.DialTimeout("tcp", testAddr, 1*time.Second)
						if err == nil {
							conn.Close()
							validAddress = testAddr
							break
						}
					}

					// If no valid address was found, skip this entry
					if validAddress == "" {
						if len(entry.AddrIPv4) > 0 {
							validAddress = net.JoinHostPort(entry.AddrIPv4[0].String(), fmt.Sprint(entry.Port))
						} else {
							continue
						}
					}

					var fingerprint string
					for _, txt := range entry.Text {
						if strings.HasPrefix(txt, "fp=") {
							fingerprint = strings.TrimPrefix(txt, "fp=")
							break
						}
					}

					prMap := registry.GetAllPeers()
					if _, exists := prMap[entry.Instance]; !exists {
						slog.Info("found remote peer", "peer", entry.Instance, "ip", validAddress)
					}

					registry.AddPeer(entry.Instance, validAddress, fingerprint)
				}
			}
		}(entries)

		err = resolver.Browse(browseCtx, "_baleen._tcp", "local.", entries)
		if err != nil {
			slog.Error("failed to browse network", "error", err)
		}

		select {
		case <-ctx.Done():
			cancel()
			return
		case <-browseCtx.Done():
			cancel()
		}
	}
}

// Additional methods for API integration
func (pr *PeerRegistry) AddCustomPeer(name, address, source string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.nodes[name] = address
	pr.metadata[name] = &PeerMeta{Source: source, Status: "reachable", Arch: "unknown", Fingerprint: "", LastSeen: time.Now()}
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
	Address     string
	Source      string
	Status      string
	Arch        string
	Fingerprint string
	LastSeen    time.Time
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
			Address:     addr,
			Source:      meta.Source,
			Status:      meta.Status,
			Arch:        meta.Arch,
			Fingerprint: meta.Fingerprint,
			LastSeen:    meta.LastSeen,
		}
	}
	return copyMap
}
