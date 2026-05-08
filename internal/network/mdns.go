package network

import (
	"context"
	"fmt"

	"github.com/grandcat/zeroconf"
)

// registers the Baleen service on the local Wi-Fi
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
func DiscoverPeers(currentNodeName string) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		fmt.Println("Failed to initialize mDNS listener:", err)
		return
	}

	// Create channel to receive data whenever a new node is found
	entries := make(chan *zeroconf.ServiceEntry)

	// Start background run to process the incoming discoveries
	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			if entry.Instance != currentNodeName {
				fmt.Printf("\nFound Remote Peer: %s (IP: %v, Port: %d)\n", entry.Instance, entry.AddrIPv4, entry.Port)
			}
		}
	}(entries)

	ctx := context.Background()
	// Start browsing
	err = resolver.Browse(ctx, "_baleen._tcp", "local.", entries)
	if err != nil {
		fmt.Println("Failed to browse network:", err)
	}
}