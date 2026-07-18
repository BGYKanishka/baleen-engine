package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/BGYKanishka/baleen-engine/internal/cli"
	"github.com/BGYKanishka/baleen-engine/internal/config"
)

func NodeName(ctx cli.EngineContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {

		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]string{"name": ctx.NodeName})

		case http.MethodPost:
			var req struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
				return
			}
			name := strings.TrimSpace(req.Name)
			if name == "" {
				http.Error(w, `{"error":"name cannot be empty"}`, http.StatusBadRequest)
				return
			}
			if err := config.SaveNodeName(name); err != nil {
				http.Error(w, `{"error":"failed to save name"}`, http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{
				"status": "saved",
				"name":   name,
			})

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
