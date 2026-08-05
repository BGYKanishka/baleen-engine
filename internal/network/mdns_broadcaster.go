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

	var server *zeroconf.Server
	var err error

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if server != nil {
				server.Shutdown()
			}
			return
		case <-ticker.C:
			isEn := enabled.Load()
			if isEn && server == nil {
				server, err = zeroconf.Register(nodeName, "_baleen._tcp", "local.", port, []string{"fp=" + fingerprint}, nil)
				if err != nil {
					slog.Error("failed to register mDNS service", "error", err)
				} else {
					slog.Info("mDNS broadcast registered")
				}
			} else if !isEn && server != nil {
				server.Shutdown()
				server = nil
				slog.Info("mDNS broadcast unregistered (disabled)")
			}
		}
	}
}
