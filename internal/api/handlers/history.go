package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/BGYKanishka/baleen-engine/internal/cli"
	"github.com/BGYKanishka/baleen-engine/internal/ledger"
)

func History(ctx cli.EngineContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		history, err := ctx.EngineLedger.GetHistory()
		if err != nil {
			http.Error(w, "Failed to retrieve ledger", http.StatusInternalServerError)
			return
		}

		if history == nil {
			history = []ledger.Commit{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(history)
	}
}
