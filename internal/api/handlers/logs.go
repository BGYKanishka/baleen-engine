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
		var lines []string
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
