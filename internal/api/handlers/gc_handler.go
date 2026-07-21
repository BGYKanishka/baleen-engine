package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/cli"
)

// GC handles garbage collection requests to clean up the ledger and optionally remove physical tar files.
func GC(ctx cli.EngineContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		var payload struct {
			Mode        string `json:"mode"`
			Days        int    `json:"days"`
			Hash        string `json:"hash"`
			RemoveCache bool   `json:"removeCache"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}

		var msg string
		switch payload.Mode {
		case "all":
			if err := ctx.EngineLedger.ClearLedgerOnly(); err != nil {
				http.Error(w, "failed to clear ledger: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if payload.RemoveCache {
				ctx.EngineLedger.ClearCacheMemory()
				freed := gcDeletePhysical(ctx.TempDir, "", time.Time{})
				msg = fmt.Sprintf("Ledger cleared and %d MB of physical data deleted.", freed)
			} else {
				msg = "Ledger history completely wiped."
			}

		case "old":
			days := payload.Days
			if days <= 0 {
				days = 7
			}
			cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
			count, err := ctx.EngineLedger.PruneHistoryOlderThan(cutoff)
			if err != nil {
				http.Error(w, "failed to prune ledger: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if payload.RemoveCache {
				freed := gcDeletePhysical(ctx.TempDir, "", cutoff)
				msg = fmt.Sprintf("Removed %d entries older than %d days and %d MB of files.", count, days, freed)
			} else {
				msg = fmt.Sprintf("Removed %d entries older than %d days.", count, days)
			}

		case "hash":
			if payload.Hash == "" {
				http.Error(w, "hash is required for mode=hash", http.StatusBadRequest)
				return
			}
			if err := ctx.EngineLedger.DeleteCommit(payload.Hash); err != nil {
				http.Error(w, "failed to delete commit: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if payload.RemoveCache {
				freed := gcDeletePhysical(ctx.TempDir, payload.Hash, time.Time{})
				msg = fmt.Sprintf("Commit %s deleted and %d MB freed.", payload.Hash, freed)
			} else {
				msg = fmt.Sprintf("Commit %s deleted.", payload.Hash)
			}

		default:
			http.Error(w, `invalid mode; use "all", "old", or "hash"`, http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": msg})
	}
}

// removes .tar files from dir matching the hash filter and/or older than cutoff.
func gcDeletePhysical(dir, hashFilter string, cutoff time.Time) int64 {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	var freedBytes int64
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".tar") {
			continue
		}
		if hashFilter != "" && !strings.Contains(entry.Name(), hashFilter) {
			continue
		}
		filePath := filepath.Join(dir, entry.Name())
		info, err := os.Stat(filePath)
		if err != nil {
			continue
		}
		if !cutoff.IsZero() && info.ModTime().After(cutoff) {
			continue
		}
		freedBytes += info.Size()
		os.RemoveAll(filePath)
	}
	return freedBytes / 1024 / 1024
}
