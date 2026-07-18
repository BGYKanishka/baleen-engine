package network

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/config"
)

// attempts to detect the architecture of a remote Baleen node by querying its metadata server.
func DetectRemoteArch(ip string, port int) string {
	metadataURL := fmt.Sprintf("http://%s:%d/architecture", ip, port+config.MetadataPortOffset)
	client := http.Client{Timeout: 2 * time.Second}

	resp, err := client.Get(metadataURL)
	if err != nil {
		return "linux/amd64"
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil || len(bodyBytes) == 0 {
		return "linux/amd64"
	}

	return strings.TrimSpace(string(bodyBytes))
}

// runs a lightweight HTTP server to expose the node's architecture.
func StartMetadataServer(port int, broadcastEnabled *atomic.Bool) {
	mux := http.NewServeMux()
	mux.HandleFunc("/architecture", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("linux/" + runtime.GOARCH))
	})

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		status := "broadcasting"
		if broadcastEnabled != nil && !broadcastEnabled.Load() {
			status = "hidden"
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status": "%s"}`, status)
	})

	addr := fmt.Sprintf(":%d", port)
	slog.Info("starting metadata server", "port", port)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("metadata server failed", "error", err)
	}
}

// ResolveTargetAddress resolves a peer name or address to an IP and port, using the peer registry if available.
func ResolveTargetAddress(registry *PeerRegistry, peer string) (ip string, port int, fingerprint string, err error) {
	address := peer

	if peers := registry.GetDetailedPeers(); len(peers) > 0 {
		if resolved, exists := peers[peer]; exists {
			address = resolved.Address
			fingerprint = resolved.Fingerprint
		}
	}

	if strings.Contains(address, ":") {
		parts := strings.SplitN(address, ":", 2)
		ip = parts[0]
		if _, scanErr := fmt.Sscanf(parts[1], "%d", &port); scanErr != nil || port <= 0 {
			err = fmt.Errorf("invalid port in address %q: %s", address, parts[1])
		}
		return
	}
	err = fmt.Errorf("no port specified for %q — use the format host:port (e.g. 192.168.1.5:52341)", peer)
	ip = address
	return
}
