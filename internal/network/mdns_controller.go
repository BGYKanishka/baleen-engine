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

	mu          sync.RWMutex
	nodeName    string
	port        int
	fingerprint string
	registry    *PeerRegistry
	parentCtx   context.Context
	cancelCtx   context.CancelFunc
	wg          *sync.WaitGroup
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
	nc := &NetworkController{
		nodeName:    nodeName,
		port:        port,
		fingerprint: fingerprint,
		registry:    registry,
		parentCtx:   parentCtx,
	}
	nc.discoveryEnabled.Store(discoveryEnabled)
	nc.broadcastEnabled.Store(broadcastEnabled)

	nc.startRoutines()

	return nc
}

func (nc *NetworkController) startRoutines() {
	ctx, cancel := context.WithCancel(nc.parentCtx)
	nc.cancelCtx = cancel
	nc.wg = &sync.WaitGroup{}
	nc.wg.Add(2)

	go DiscoverPeers(ctx, nc.wg, nc.nodeName, nc.registry, &nc.discoveryEnabled)
	go StartBroadcaster(ctx, nc.wg, nc.nodeName, nc.port, nc.fingerprint, &nc.broadcastEnabled)
}

// updates the node name and restarts mDNS routines if the name changed.
func (nc *NetworkController) UpdateNodeName(newName string) {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	if nc.nodeName == newName {
		return
	}
	nc.nodeName = newName

	if nc.cancelCtx != nil {
		nc.cancelCtx()
		nc.wg.Wait()
	}
	nc.startRoutines()
}

// returns the current node name safely.
func (nc *NetworkController) GetNodeName() string {
	nc.mu.RLock()
	defer nc.mu.RUnlock()
	return nc.nodeName
}

// enables or disables peer discovery. Takes effect within 500 ms.
func (nc *NetworkController) SetDiscovery(enabled bool) {
	prev := nc.discoveryEnabled.Swap(enabled)
	if prev != enabled {
		slog.Info("mDNS discovery toggled", "enabled", enabled)
		if !enabled && nc.registry != nil {
			nc.registry.ClearMDNSPeers()
		}
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
