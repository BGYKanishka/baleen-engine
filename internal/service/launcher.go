package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/BGYKanishka/baleen-engine/internal/config"
)

// BackgroundEnvKey is the environment variable that distinguishes the real
// background daemon from the short-lived launcher process.
const BackgroundEnvKey = "BALEEN_BACKGROUND"

// returns true when the current process is the real background daemon,
// false when it's the short-lived launcher process.
func IsBackgroundProcess() bool {
	return os.Getenv(BackgroundEnvKey) == "1"
}

// returns the path to the daemon log file.
func daemonLogPath() (string, error) {
	baleenRoot, err := config.BaleenDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baleenRoot, "daemon.log"), nil
}

// LaunchBackground launches the background daemon process, which will run
// independently of the current process. The daemon will write its logs to a
// file in the user's home directory.
func LaunchBackground(token string, name string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	logPath, err := daemonLogPath()
	if err != nil {
		return fmt.Errorf("resolve log path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return fmt.Errorf("create baleen directory: %w", err)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open daemon log: %w", err)
	}

	cmd := exec.Command(exePath, "daemon", "--token", token, "--name", name)
	cmd.Env = append(os.Environ(), BackgroundEnvKey+"=1")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil

	setProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start background process: %w", err)
	}
	logFile.Close()
	return nil
}
