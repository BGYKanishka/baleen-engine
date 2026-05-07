package network

import (
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