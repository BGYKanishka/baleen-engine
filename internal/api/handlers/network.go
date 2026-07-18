package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/BGYKanishka/baleen-engine/internal/cli"
	"github.com/BGYKanishka/baleen-engine/internal/config"
)

// returns an HTTP handler that allows getting and setting network settings.
func NetworkSettings(ctx cli.EngineContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		nc := ctx.NetworkController
		if nc == nil {
			http.Error(w, `{"error":"network controller unavailable"}`, http.StatusServiceUnavailable)
			return
		}

		switch r.Method {

		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]bool{
				"mdns_discovery":     nc.IsDiscoveryEnabled(),
				"broadcast_presence": nc.IsBroadcastEnabled(),
			})

		case http.MethodPost:
			// Accept a partial JSON body: only the keys present will be applied.
			var req struct {
				MDNSDiscovery     *bool `json:"mdns_discovery"`
				BroadcastPresence *bool `json:"broadcast_presence"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
				return
			}

			if req.MDNSDiscovery != nil {
				nc.SetDiscovery(*req.MDNSDiscovery)
				// Immediately evict cached mDNS peers so the peer list clears
				// at once instead of waiting for the health-checker timeout.
				if !*req.MDNSDiscovery {
					ctx.PeerRegistry.ClearMDNSPeers()
				}
			}
			if req.BroadcastPresence != nil {
				nc.SetBroadcast(*req.BroadcastPresence)
			}

			// Persist the new state so it survives daemon restarts.
			updated := config.NetworkSettings{
				MDNSDiscovery:     nc.IsDiscoveryEnabled(),
				BroadcastPresence: nc.IsBroadcastEnabled(),
			}
			if err := config.SaveNetworkSettings(updated); err != nil {
				// Settings were applied but could not be persisted. Return a warning to the client.
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{
					"mdns_discovery":     updated.MDNSDiscovery,
					"broadcast_presence": updated.BroadcastPresence,
					"warning":            "settings applied but could not be persisted: " + err.Error(),
				})
				return
			}

			json.NewEncoder(w).Encode(map[string]bool{
				"mdns_discovery":     updated.MDNSDiscovery,
				"broadcast_presence": updated.BroadcastPresence,
			})

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
