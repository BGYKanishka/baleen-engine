package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/BGYKanishka/baleen-engine/internal/updater"
	"github.com/BGYKanishka/baleen-engine/internal/version"
)

func UpdateCheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		force := r.URL.Query().Get("force") == "true"
		res, err := updater.CheckForUpdate(version.Version, force)
		if err != nil {
			log.Printf("update check failed: %v", err)
			res = updater.UpdateCheckResult{
				UpdateAvailable: false,
				LatestVersion:   version.Version,
				CurrentVersion:  version.Version,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(res); err != nil {
			log.Printf("failed to encode update response: %v", err)
		}
	}
}
