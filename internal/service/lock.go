package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func lockPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".baleen", "daemon.lock"), nil
}

// attempts to atomically claim the lock file using O_EXCL.
// Returns true if this process is now the owner, false if another live process
// already holds the lock.
func TryAcquireLock() (bool, error) {
	path, err := lockPath()
	if err != nil {
		return false, err
	}

	// Try atomic exclusive create first.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if !os.IsExist(err) {
			return false, fmt.Errorf("create lock file: %w", err)
		}

		// File already exists — check if the holder is still alive.
		if data, rerr := os.ReadFile(path); rerr == nil {
			pidStr := strings.TrimSpace(string(data))
			if pid, perr := strconv.Atoi(pidStr); perr == nil && pid > 0 {
				if proc, perr := os.FindProcess(pid); perr == nil && isProcessAlive(proc) {
					return false, nil // live holder — don't take the lock
				}
			}
		}

		// Stale lock (holder dead) — remove it and try again.
		os.Remove(path)
		f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			// Another process grabbed it in the tiny race window — that's fine.
			return false, nil
		}
	}

	defer f.Close()
	fmt.Fprintf(f, "%d", os.Getpid())
	return true, nil
}

// removes the lock file. Call on clean shutdown.
func ReleaseLock() {
	path, _ := lockPath()
	os.Remove(path)
}
