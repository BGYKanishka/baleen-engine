package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/BGYKanishka/baleen-engine/internal/cli"
	"github.com/BGYKanishka/baleen-engine/internal/config"
)

func ResetSettings(ctx cli.EngineContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		// Reset Network Settings
		networkDefaults := config.DefaultNetworkSettings()
		if nc := ctx.NetworkController; nc != nil {
			nc.SetDiscovery(networkDefaults.MDNSDiscovery)
			if !networkDefaults.MDNSDiscovery {
				ctx.PeerRegistry.ClearMDNSPeers()
			}
			nc.SetBroadcast(networkDefaults.BroadcastPresence)
		}
		if err := config.SaveNetworkSettings(networkDefaults); err != nil {
			http.Error(w, `{"error":"failed to save network settings: `+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		// Reset Transfer Settings
		transferDefaults := config.DefaultTransferSettings()
		if err := config.SaveTransferSettings(transferDefaults); err != nil {
			http.Error(w, `{"error":"failed to save transfer settings: `+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		// Clear Node Name
		if err := config.SaveNodeName(""); err != nil {
			http.Error(w, `{"error":"failed to clear node name: `+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]any{
			"status":   "success",
			"network":  networkDefaults,
			"transfer": transferDefaults,
		})
	}
}
