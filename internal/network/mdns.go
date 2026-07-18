package network

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/config"
	"github.com/grandcat/zeroconf"
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

// constantly pings known peers to ensure they are still online
func (pr *PeerRegistry) StartHealthChecker(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
		pr.mu.RLock()
		checkList := make(map[string]string)
		for name, addr := range pr.nodes {
			checkList[name] = addr
		}
		pr.mu.RUnlock()

		for name, addr := range checkList {
			go func(name, addr string) {
				// Check if the peer is broadcasting its presence via HTTP status endpoint
				host, portStr, err := net.SplitHostPort(addr)
				hidden := false
				if err == nil {
					port, _ := strconv.Atoi(portStr)
					metadataURL := fmt.Sprintf("http://%s:%d/status", host, port+config.MetadataPortOffset)
					client := http.Client{Timeout: 1 * time.Second}
					if resp, err := client.Get(metadataURL); err == nil {
						bodyBytes, _ := io.ReadAll(resp.Body)
						resp.Body.Close()
						if strings.Contains(string(bodyBytes), `"hidden"`) {
							hidden = true
						}
					}
				}

				if hidden {
					pr.RemovePeer(name)
					slog.Info("peer stopped broadcasting, evicted", "peer", name)
					return
				}

				// Ping the peer's TCP port to check reachability
				conn, err := net.DialTimeout("tcp", addr, 1*time.Second)

				pr.mu.Lock()
				meta, exists := pr.metadata[name]
				if !exists {
					pr.mu.Unlock()
					return
				}

				if err != nil {
					meta.Status = "unreachable"
					if meta.Source != "static" && time.Since(meta.LastSeen) > 60*time.Second {
						pr.mu.Unlock()
						pr.RemovePeer(name)
						slog.Info("peer disconnected", "peer", name)
						return
					}
				} else {
					conn.Close()
					meta.Status = "reachable"
				}
				pr.mu.Unlock()
			}(name, addr)
		}
	}
}

// advertises this node via mDNS. The enabled flag is checked every 200 ms
func StartBroadcaster(ctx context.Context, wg *sync.WaitGroup, nodeName string, port int, fingerprint string, enabled *atomic.Bool) {
	defer wg.Done()
	slog.Info("broadcast controller started", "nodeName", nodeName, "port", port)

	for {
		// Respect daemon shutdown.
		select {
		case <-ctx.Done():
			return
		default:
		}

		if !enabled.Load() {
			select {
			case <-ctx.Done():
				return
			case <-time.After(200 * time.Millisecond):
			}
			continue
		}

		// Register the mDNS service.
		server, err := zeroconf.Register(nodeName, "_baleen._tcp", "local.", port, []string{"fp=" + fingerprint}, nil)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}

		// Hold the registration, checking every 200 ms for disable or shutdown.
		ticker := time.NewTicker(200 * time.Millisecond)
		deadline := time.NewTimer(30 * time.Second)
	holdLoop:
		for {
			select {
			case <-ctx.Done():
				server.Shutdown()
				ticker.Stop()
				deadline.Stop()
				return
			case <-deadline.C:
				server.Shutdown()
				ticker.Stop()
				break holdLoop
			case <-ticker.C:
				if !enabled.Load() {
					server.Shutdown()
					slog.Info("broadcast presence disabled")
					ticker.Stop()
					deadline.Stop()
					break holdLoop
				}
			}
		}
	}
}

// continuously sweeps the local network for Baleen nodes.
func DiscoverPeers(ctx context.Context, wg *sync.WaitGroup, currentNodeName string, registry *PeerRegistry, enabled *atomic.Bool) {
	defer wg.Done()
	for {
		// Respect daemon shutdown.
		select {
		case <-ctx.Done():
			return
		default:
		}

		if !enabled.Load() {
			// Discovery disabled — idle briefly and recheck.
			select {
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
			}
			continue
		}

		// Create new resolver every cycle
		resolver, err := zeroconf.NewResolver(nil)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
				continue
			}
		}

		entries := make(chan *zeroconf.ServiceEntry)
		browseCtx, cancel := context.WithTimeout(ctx, 15*time.Second)

		go func(results <-chan *zeroconf.ServiceEntry) {
			for entry := range results {
				if !enabled.Load() {
					continue
				}
				if entry.Instance == currentNodeName {
					continue
				}

				// Ping all IPs and keep the reachable one.
				var validAddress string
				for _, ipAddr := range entry.AddrIPv4 {
					testAddr := net.JoinHostPort(ipAddr.String(), fmt.Sprint(entry.Port))
					conn, err := net.DialTimeout("tcp", testAddr, 1*time.Second)
					if err == nil {
						conn.Close()
						validAddress = testAddr
						break
					}
				}
				if validAddress == "" {
					if len(entry.AddrIPv4) > 0 {
						validAddress = net.JoinHostPort(entry.AddrIPv4[0].String(), fmt.Sprint(entry.Port))
					} else {
						continue
					}
				}

				var fingerprint string
				for _, txt := range entry.Text {
					if strings.HasPrefix(txt, "fp=") {
						fingerprint = strings.TrimPrefix(txt, "fp=")
						break
					}
				}

				prMap := registry.GetAllPeers()
				if _, exists := prMap[entry.Instance]; !exists {
					slog.Info("found remote peer", "peer", entry.Instance, "ip", validAddress)
				}
				registry.AddPeer(entry.Instance, validAddress, fingerprint)
			}
		}(entries)

		if err := resolver.Browse(browseCtx, "_baleen._tcp", "local.", entries); err != nil {
			slog.Error("failed to browse network", "error", err)
		}

		// Wait for the browse context to finish or for shutdown,
		// checking every 200 ms if discovery is still enabled.
		pollTicker := time.NewTicker(200 * time.Millisecond)
	waitLoop:
		for {
			select {
			case <-ctx.Done():
				pollTicker.Stop()
				cancel()
				return
			case <-browseCtx.Done():
				pollTicker.Stop()
				cancel()
				break waitLoop
			case <-pollTicker.C:
				if !enabled.Load() {
					pollTicker.Stop()
					cancel()
					break waitLoop
				}
			}
		}
	}
}

// NetworkController

// NetworkController manages the mDNS discovery and broadcast goroutines, allowing them to be toggled on/off at runtime.
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

// Additional methods for API integration
func (pr *PeerRegistry) AddCustomPeer(name, address, source string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
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
