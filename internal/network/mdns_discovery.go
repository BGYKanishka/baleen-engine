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
