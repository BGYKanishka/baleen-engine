package network

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
)

// manages the mDNS discovery and broadcast goroutines, allowing them to be toggled on/off at runtime.
type NetworkController struct {
	discoveryEnabled atomic.Bool
	broadcastEnabled atomic.Bool
}

// starts both mDNS goroutines immediately and sets their
// initial enabled states from the persisted settings.
func NewNetworkController(
	parentCtx context.Context,
	nodeName string,
	port int,
	fingerprint string,
	registry *PeerRegistry,
	discoveryEnabled bool,
	broadcastEnabled bool,
) *NetworkController {
	nc := &NetworkController{}
	nc.discoveryEnabled.Store(discoveryEnabled)
	nc.broadcastEnabled.Store(broadcastEnabled)

	var wg sync.WaitGroup
	wg.Add(2)
	go DiscoverPeers(parentCtx, &wg, nodeName, registry, &nc.discoveryEnabled)
	go StartBroadcaster(parentCtx, &wg, nodeName, port, fingerprint, &nc.broadcastEnabled)

	return nc
}

// enables or disables peer discovery. Takes effect within 500 ms.
func (nc *NetworkController) SetDiscovery(enabled bool) {
	prev := nc.discoveryEnabled.Swap(enabled)
	if prev != enabled {
		slog.Info("mDNS discovery toggled", "enabled", enabled)
	}
}

// enables or disables presence broadcasting. Takes effect within 200 ms.
func (nc *NetworkController) SetBroadcast(enabled bool) {
	prev := nc.broadcastEnabled.Swap(enabled)
	if prev != enabled {
		slog.Info("broadcast presence toggled", "enabled", enabled)
	}
}

// reports whether mDNS discovery is currently enabled.
func (nc *NetworkController) IsDiscoveryEnabled() bool {
	return nc.discoveryEnabled.Load()
}

// reports whether presence broadcasting is currently enabled.
func (nc *NetworkController) IsBroadcastEnabled() bool {
	return nc.broadcastEnabled.Load()
}

// returns a pointer to the atomic boolean controlling broadcast presence.
func (nc *NetworkController) BroadcastEnabledFlag() *atomic.Bool {
	return &nc.broadcastEnabled
}
