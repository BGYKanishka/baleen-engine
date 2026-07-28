package network

import (
	"os"
	"testing"
)

func TestPeerRegistry_AddPeer(t *testing.T) {
	pr := NewPeerRegistry()

	pr.AddPeer("node-1", "192.168.1.10:8080", "fingerprint1")
	
	peers := pr.GetAllPeers()
	if len(peers) != 1 || peers["node-1"] != "192.168.1.10:8080" {
		t.Fatalf("expected node-1 to be added, got: %v", peers)
	}

	details := pr.GetDetailedPeers()
	if details["node-1"].Source != "mdns" {
		t.Errorf("expected source mdns, got %s", details["node-1"].Source)
	}
	if details["node-1"].Fingerprint != "fingerprint1" {
		t.Errorf("expected fingerprint fingerprint1, got %s", details["node-1"].Fingerprint)
	}

	// Test IP clash (same address, different name)
	pr.AddPeer("node-1-renamed", "192.168.1.10:8080", "fingerprint1")
	peers = pr.GetAllPeers()
	if len(peers) != 1 {
		t.Fatalf("expected old name to be deleted on IP clash, got %d peers: %v", len(peers), peers)
	}
	if peers["node-1-renamed"] != "192.168.1.10:8080" {
		t.Fatalf("expected node-1-renamed, got: %v", peers)
	}
	
	details = pr.GetDetailedPeers()
	if _, exists := details["node-1"]; exists {
		t.Error("node-1 metadata should have been deleted")
	}
}

func TestPeerRegistry_AddCustomPeer(t *testing.T) {
	pr := NewPeerRegistry()

	pr.AddCustomPeer("manual-node", "10.0.0.5:9090", "manual")
	
	details := pr.GetDetailedPeers()
	if len(details) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(details))
	}
	if details["manual-node"].Source != "manual" {
		t.Errorf("expected source manual, got %s", details["manual-node"].Source)
	}
	if details["manual-node"].Address != "10.0.0.5:9090" {
		t.Errorf("expected address 10.0.0.5:9090, got %s", details["manual-node"].Address)
	}
}

func TestPeerRegistry_ClearMDNSPeers(t *testing.T) {
	pr := NewPeerRegistry()

	pr.AddPeer("mdns-node", "192.168.1.50:8080", "fp-mdns")
	pr.AddCustomPeer("manual-node", "10.0.0.5:9090", "manual")
	pr.AddCustomPeer("static-node", "10.0.0.6:9090", "static")

	if len(pr.GetAllPeers()) != 3 {
		t.Fatalf("expected 3 peers initially")
	}

	pr.ClearMDNSPeers()

	peers := pr.GetAllPeers()
	if len(peers) != 2 {
		t.Fatalf("expected 2 peers after clearing mDNS, got %d", len(peers))
	}

	if _, exists := peers["mdns-node"]; exists {
		t.Error("mdns-node should have been cleared")
	}
	if _, exists := peers["manual-node"]; !exists {
		t.Error("manual-node should NOT have been cleared")
	}
	if _, exists := peers["static-node"]; !exists {
		t.Error("static-node should NOT have been cleared")
	}
	
	details := pr.GetDetailedPeers()
	if _, exists := details["mdns-node"]; exists {
		t.Error("mdns-node metadata should have been cleared")
	}
}

func TestPeerRegistry_RemovePeer(t *testing.T) {
	pr := NewPeerRegistry()
	pr.AddPeer("node-1", "192.168.1.10:8080", "fp")
	
	if len(pr.GetAllPeers()) != 1 {
		t.Fatal("expected 1 peer")
	}

	pr.RemovePeer("node-1")
	
	if len(pr.GetAllPeers()) != 0 {
		t.Fatal("expected 0 peers after removal")
	}
	if len(pr.GetDetailedPeers()) != 0 {
		t.Fatal("expected 0 metadata entries after removal")
	}
}

func TestPeerRegistry_LoadStaticPeers(t *testing.T) {
	pr := NewPeerRegistry()

	os.Setenv("BALEEN_PEERS", "192.168.2.10:8080, 192.168.2.11:8080 ")
	defer os.Unsetenv("BALEEN_PEERS")

	LoadStaticPeers(pr)

	peers := pr.GetAllPeers()
	if len(peers) != 2 {
		t.Fatalf("expected 2 static peers, got %d", len(peers))
	}

	details := pr.GetDetailedPeers()
	p1 := details["static-peer-1"]
	p2 := details["static-peer-2"]

	if p1 == nil || p1.Address != "192.168.2.10:8080" || p1.Source != "static" {
		t.Errorf("invalid static-peer-1: %+v", p1)
	}
	if p2 == nil || p2.Address != "192.168.2.11:8080" || p2.Source != "static" {
		t.Errorf("invalid static-peer-2: %+v", p2)
	}
}
