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

func StartBroadcaster(nodeName string, port int) {
	server, err := zeroconf.Register(nodeName, "_baleen._tcp", "local.", port, nil, nil)
	if err != nil {
		panic(fmt.Errorf("failed to start mDNS broadcaster: %w", err))
	}
	defer server.Shutdown()

	fmt.Printf("Broadcasting presence on local WiFi as '%s' (Port %d)...\n", nodeName, port)
	select {}
}

// constantly scans the Wi-Fi network for other Baleen nodes
func DiscoverPeers(currentNodeName string, registry *PeerRegistry) {
	go func() {
		for {
			resolver, err := zeroconf.NewResolver(nil)
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}

			entries := make(chan *zeroconf.ServiceEntry)

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

			go func(results <-chan *zeroconf.ServiceEntry) {
				for entry := range results {
					if entry.Instance != currentNodeName {
						var ip string
						if len(entry.AddrIPv4) > 0 {
							ip = entry.AddrIPv4[0].String()
						} else {
							continue
						}
						address := fmt.Sprintf("%s:%d", ip, entry.Port)

						// Check our registry to see if we need to welcome them
						prMap := registry.GetAllPeers()
						if _, exists := prMap[entry.Instance]; !exists {
							fmt.Printf("\n [Discovery] Found Remote Peer: %s (IP: %s)\n", entry.Instance, address)
							fmt.Print("baleen> ")
						}

						// Refresh them
						registry.AddPeer(entry.Instance, address)
					}
				}
			}(entries)

			// Start the sweep
			err = resolver.Browse(ctx, "_baleen._tcp", "local.", entries)
			if err != nil {
				fmt.Println("Failed to browse network:", err)
			}

			// Block until the 15 seconds are up
			<-ctx.Done()
			cancel()

		}
	}()
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

		// grab a snapshot of current peers
		pr.mu.RLock()
		checkList := make(map[string]string)
		for name, addr := range pr.nodes {
			checkList[name] = addr
		}
		pr.mu.RUnlock()

		// Ping each peer
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
