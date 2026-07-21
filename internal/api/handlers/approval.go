package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/BGYKanishka/baleen-engine/internal/cli"
)

func Pending(ctx cli.EngineContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		req, ok := ctx.PendingApproval.Load()
		if !ok {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(req.Req)
	}
}
func Approve(ctx cli.EngineContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sendDecision(ctx, w, true)
	}
}
func Reject(ctx cli.EngineContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sendDecision(ctx, w, false)
	}
}

func sendDecision(ctx cli.EngineContext, w http.ResponseWriter, approved bool) {
	req, ok := ctx.PendingApproval.Load()
	if !ok {
		http.Error(w, "no pending approval", http.StatusConflict)
		return
	}
	ctx.PendingApproval.Clear()
	req.Response <- approved
	w.WriteHeader(http.StatusOK)
}
