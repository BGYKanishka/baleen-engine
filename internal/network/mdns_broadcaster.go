package network

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/grandcat/zeroconf"
)

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
