package network

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

// safely stores active nodes to prevent race conditions
type PeerRegistry struct {
	mu    sync.RWMutex
	nodes map[string]string
}

func NewPeerRegistry() *PeerRegistry {
	return &PeerRegistry{
		nodes: make(map[string]string),
	}
}

func (pr *PeerRegistry) AddPeer(name, address string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.nodes[name] = address
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
				fmt.Printf("\n[Network] Peer disconnected: %s\n", name)
				fmt.Print("baleen> ")
			} else {
				conn.Close()
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
							fmt.Printf("\n[Discovery] Found Remote Peer: %s (IP: %s)\n", entry.Instance, validAddress)
							fmt.Print("baleen> ")
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
