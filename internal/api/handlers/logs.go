package handlers

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/BGYKanishka/baleen-engine/internal/config"
)

// Logs returns the last 500 lines of the daemon log file as a JSON array.
func Logs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		baleenRoot, err := config.BaleenDir()
		if err != nil {
			http.Error(w, "Cannot resolve home dir", http.StatusInternalServerError)
			return
		}
		logPath := filepath.Join(baleenRoot, "daemon.log")

		file, err := os.Open(logPath)
		if err != nil {
			if os.IsNotExist(err) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`[]`))
				return
			}
			http.Error(w, "Cannot open log file", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// Read the log file line by line and keep only the last 500 lines
		scanner := bufio.NewScanner(file)
		lines := []string{}
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
			if len(lines) > 500 {
				lines = lines[1:]
			}
		}

		if err := scanner.Err(); err != nil {
			http.Error(w, "Error reading log file", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(lines)
	}
}

// truncates or deletes the daemon log file.
func CleanLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Remove bool `json:"removeCache"`
		}
		if r.Body != nil {
			json.NewDecoder(r.Body).Decode(&req)
		}

		baleenRoot, err := config.BaleenDir()
		if err != nil {
			http.Error(w, "Cannot resolve home dir", http.StatusInternalServerError)
			return
		}
		logPath := filepath.Join(baleenRoot, "daemon.log")

		if req.Remove {
			if err := os.Remove(logPath); err != nil && !os.IsNotExist(err) {
				http.Error(w, "Failed to delete log file", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"message":"Daemon log deleted"}`))
			return
		}

		// Otherwise just truncate it
		file, err := os.OpenFile(logPath, os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			if os.IsNotExist(err) {
				// already clean/missing
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"message":"Daemon log already empty"}`))
				return
			}
			http.Error(w, "Failed to truncate log file", http.StatusInternalServerError)
			return
		}
		file.Close()

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message":"Daemon log truncated"}`))
	}
}
