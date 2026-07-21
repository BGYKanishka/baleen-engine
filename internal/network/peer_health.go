package network

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/config"
)

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
				// First, attempt to check the peer's broadcast status via its metadata server.
				// If it explicitly reports as "hidden", evict it instantly.
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

				// Fallback: If HTTP check fails (e.g. old node without /status), do a raw TCP ping.
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
