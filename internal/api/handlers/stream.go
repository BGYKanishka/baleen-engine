package handlers

import (
	"net/http"

	"github.com/BGYKanishka/baleen-engine/internal/transfer"
)

// Stream returns current transfer state as a JSON array
func Stream() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(transfer.GlobalHub.GetAll())
	}
}
