package network

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/grandcat/zeroconf"
)

// continuously sweeps the local network for Baleen nodes.
func DiscoverPeers(ctx context.Context, wg *sync.WaitGroup, currentNodeName string, registry *PeerRegistry, enabled *atomic.Bool) {
	defer wg.Done()

	var resolver *zeroconf.Resolver
	var err error

	for {
		if !enabled.Load() {
			if resolver != nil {
				resolver = nil
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
			}
			continue
		}

		if resolver == nil {
			resolver, err = zeroconf.NewResolver(nil)
			if err != nil {
				slog.Error("failed to create mDNS resolver", "error", err)
				resolver = nil
				select {
				case <-ctx.Done():
					return
				case <-time.After(2 * time.Second):
				}
				continue
			}
		}

		// Run the browse session until it's disabled or the context cancels.
		runBrowseSession(ctx, resolver, currentNodeName, registry, enabled)

		// If runBrowseSession returns, either ctx is done, or enabled went false.
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

// manages a single continuous browse session.
func runBrowseSession(parentCtx context.Context, resolver *zeroconf.Resolver, currentNodeName string, registry *PeerRegistry, enabled *atomic.Bool) {
	browseCtx, cancel := context.WithCancel(parentCtx)
	defer cancel() // Satisfies the lostcancel linter

	entries := make(chan *zeroconf.ServiceEntry)

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

			if fingerprint == "" {
				slog.Warn("ignoring peer with no TLS fingerprint", "peer", entry.Instance, "ip", validAddress)
				continue
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
		return
	}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-parentCtx.Done():
			return
		case <-ticker.C:
			if !enabled.Load() {
				return // Triggers defer cancel()
			}
		}
	}
}
