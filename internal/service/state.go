package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/config"
)

// represents the state of the background service,
// including its port, token, PID, node name, and start time.
type ServiceState struct {
	Port      int       `json:"port"`
	Token     string    `json:"token"`
	PID       int       `json:"pid"`
	NodeName  string    `json:"node_name"`
	StartedAt time.Time `json:"started_at"`
}

// atomically writes the service state to ~/.baleen/service.json.
func WriteState(state ServiceState) error {
	path, err := config.ServiceStatePath()
	if err != nil {
		return fmt.Errorf("resolve state path: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	// Write to a temp file first then rename for atomicity.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write temp state: %w", err)
	}
	return os.Rename(tmp, path)
}

// reads and parses the service state file.
// Returns an error if the file does not exist.
func ReadState() (ServiceState, error) {
	path, err := config.ServiceStatePath()
	if err != nil {
		return ServiceState{}, fmt.Errorf("resolve state path: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ServiceState{}, err
	}

	var state ServiceState
	if err := json.Unmarshal(data, &state); err != nil {
		return ServiceState{}, fmt.Errorf("parse state file: %w", err)
	}
	return state, nil
}

// removes the service state file.
func ClearState() {
	path, err := config.ServiceStatePath()
	if err != nil {
		return
	}
	os.Remove(path)
}

// returns true if the process with the given PID is alive.
func IsAlive(state ServiceState) bool {
	if state.PID <= 0 {
		return false
	}
	proc, err := os.FindProcess(state.PID)
	if err != nil {
		return false
	}
	return isProcessAlive(proc)
}

// polls the state file until a valid state appears or timeout expires.
// The launcher calls this after spawning the background process.
func WaitForReady(timeout time.Duration) (ServiceState, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if state, err := ReadState(); err == nil && state.Port > 0 && IsAlive(state) {
			return state, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return ServiceState{}, fmt.Errorf("timed out after %s waiting for background service to become ready", timeout)
}

// error returned when the service is already running and cannot be started again.
var ErrAlreadyRunning = errors.New("baleen daemon is already running")
