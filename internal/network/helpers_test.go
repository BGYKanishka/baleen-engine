package network

import (
	"testing"
)

// ResolveTargetAddress
// parses a raw "host:port" when no registry entry exists.
func TestResolveTargetAddress_WithPort(t *testing.T) {
	registry := NewPeerRegistry()
	ip, port, err := ResolveTargetAddress(registry, "192.168.1.5:52341")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip != "192.168.1.5" {
		t.Errorf("ip: got %q, want %q", ip, "192.168.1.5")
	}
	if port != 52341 {
		t.Errorf("port: got %d, want %d", port, 52341)
	}
}

// resolves a peer that exists in the registry.
func TestResolveTargetAddress_KnownPeer(t *testing.T) {
	registry := NewPeerRegistry()
	registry.AddPeer("swift-whale-42", "10.0.0.5:44321")

	ip, port, err := ResolveTargetAddress(registry, "swift-whale-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip != "10.0.0.5" {
		t.Errorf("ip: got %q, want %q", ip, "10.0.0.5")
	}
	if port != 44321 {
		t.Errorf("port: got %d, want %d", port, 44321)
	}
}

// Verifies bare IP without port returns a clear error.
func TestResolveTargetAddress_BareIP_ReturnsError(t *testing.T) {
	registry := NewPeerRegistry()
	_, _, err := ResolveTargetAddress(registry, "192.168.1.5")
	if err == nil {
		t.Fatal("expected error for bare IP, got nil")
	}
	t.Logf("Correctly got error: %v", err)
}

// verifies that an unknown peer name without a port is also rejected.
func TestResolveTargetAddress_UnknownPeerNoPort(t *testing.T) {
	registry := NewPeerRegistry()
	_, _, err := ResolveTargetAddress(registry, "mystery-node")
	if err == nil {
		t.Fatal("expected error for unknown peer name with no port, got nil")
	}
}

// verifies that the registry lookup wins over treating the name as a raw address.
func TestResolveTargetAddress_KnownPeerOverridesRawIP(t *testing.T) {
	registry := NewPeerRegistry()
	registry.AddPeer("node-a", "172.16.0.1:9000")

	ip, port, err := ResolveTargetAddress(registry, "node-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip != "172.16.0.1" || port != 9000 {
		t.Errorf("wrong resolution: got %s:%d", ip, port)
	}
}

// PeerRegistry
// verifies basic add/retrieve behaviour.
func TestPeerRegistry_AddAndGet(t *testing.T) {
	r := NewPeerRegistry()
	r.AddPeer("alpha", "10.0.0.1:1234")
	r.AddPeer("beta", "10.0.0.2:5678")

	peers := r.GetAllPeers()
	if len(peers) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(peers))
	}
	if peers["alpha"] != "10.0.0.1:1234" {
		t.Errorf("alpha address wrong: %s", peers["alpha"])
	}
}

// verifies a disconnected peer is deleted.
func TestPeerRegistry_Remove(t *testing.T) {
	r := NewPeerRegistry()
	r.AddPeer("alpha", "10.0.0.1:1234")
	r.RemovePeer("alpha")

	peers := r.GetAllPeers()
	if _, exists := peers["alpha"]; exists {
		t.Error("expected alpha to be removed from registry")
	}
}

// verifies that mutating the returned map does not affect the registry.
func TestPeerRegistry_GetAllPeers_ReturnsCopy(t *testing.T) {
	r := NewPeerRegistry()
	r.AddPeer("gamma", "10.0.0.3:9999")

	snapshot := r.GetAllPeers()
	delete(snapshot, "gamma") // mutate the copy

	peers := r.GetAllPeers()
	if _, exists := peers["gamma"]; !exists {
		t.Error("modifying returned map should not affect the registry")
	}
}

// verifies the registry doesn't race under concurrent reads and writes.
func TestPeerRegistry_ConcurrentAccess(t *testing.T) {
	r := NewPeerRegistry()
	done := make(chan struct{})

	go func() {
		for i := 0; i < 100; i++ {
			r.AddPeer("concurrent-node", "10.0.0.99:8888")
		}
		close(done)
	}()

	for i := 0; i < 100; i++ {
		_ = r.GetAllPeers()
	}
	<-done
}
