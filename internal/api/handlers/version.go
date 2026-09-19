package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/BGYKanishka/baleen-engine/internal/version"
)

func Version() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]string{"version": version.Version}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("failed to encode version response: %v", err)
		}
	}
}
