package network

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
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

	// If another peer has the exact same IP/address under a different name, remove it.
	for existingName, existingAddr := range pr.nodes {
		if existingAddr == address && existingName != name {
			delete(pr.nodes, existingName)
			delete(pr.metadata, existingName)
		}
	}

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
	delete(pr.metadata, name)
}

// clears all peers from the registry, leaving only static or manually added peers.
func (pr *PeerRegistry) ClearMDNSPeers() {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	for name, meta := range pr.metadata {
		if meta != nil && meta.Source == "mdns" {
			delete(pr.nodes, name)
			delete(pr.metadata, name)
		}
	}
	slog.Info("cleared all mDNS-discovered peers from registry")
}

// Additional methods for API integration

func (pr *PeerRegistry) AddCustomPeer(name, address, source string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	for existingName, existingAddr := range pr.nodes {
		if existingAddr == address && existingName != name {
			delete(pr.nodes, existingName)
			delete(pr.metadata, existingName)
		}
	}

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
