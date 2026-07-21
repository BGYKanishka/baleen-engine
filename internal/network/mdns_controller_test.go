package network

import (
	"context"
	"testing"
)

func TestNetworkController(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := NewPeerRegistry()
	nc := NewNetworkController(ctx, "test-node", 9999, "test-fingerprint", reg, true, false)

	// Check initial state
	if !nc.IsDiscoveryEnabled() {
		t.Error("expected discovery to be true initially")
	}
	if nc.IsBroadcastEnabled() {
		t.Error("expected broadcast to be false initially")
	}

	// Toggle discovery
	nc.SetDiscovery(false)
	if nc.IsDiscoveryEnabled() {
		t.Error("expected discovery to be false after toggle")
	}

	// Toggle broadcast
	nc.SetBroadcast(true)
	if !nc.IsBroadcastEnabled() {
		t.Error("expected broadcast to be true after toggle")
	}

	// Check BroadcastEnabledFlag pointer
	flag := nc.BroadcastEnabledFlag()
	if !flag.Load() {
		t.Error("expected BroadcastEnabledFlag pointer to read true")
	}
}
