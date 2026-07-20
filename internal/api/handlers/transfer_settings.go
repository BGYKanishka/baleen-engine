package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/BGYKanishka/baleen-engine/internal/config"
)

// returns an HTTP handler that allows getting and setting transfer settings.
func TransferSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			settings := config.LoadTransferSettings()
			json.NewEncoder(w).Encode(settings)

		case http.MethodPost:
			// Accept a partial JSON body
			var req struct {
				AutoApprove  *bool `json:"auto_approve"`
				MaxBandwidth *int  `json:"max_bandwidth"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
				return
			}

			settings := config.LoadTransferSettings()

			if req.AutoApprove != nil {
				settings.AutoApprove = *req.AutoApprove
			}
			if req.MaxBandwidth != nil {
				settings.MaxBandwidth = *req.MaxBandwidth
			}

			if err := config.SaveTransferSettings(settings); err != nil {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{
					"auto_approve":  settings.AutoApprove,
					"max_bandwidth": settings.MaxBandwidth,
					"warning":       "settings applied but could not be persisted: " + err.Error(),
				})
				return
			}

			json.NewEncoder(w).Encode(settings)
		}
	}
}
