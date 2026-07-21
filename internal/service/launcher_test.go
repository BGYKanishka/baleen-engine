package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsBackgroundProcess(t *testing.T) {
	// By default it should be false
	t.Setenv(BackgroundEnvKey, "")
	if IsBackgroundProcess() {
		t.Error("expected IsBackgroundProcess to be false")
	}

	// Set to 1
	t.Setenv(BackgroundEnvKey, "1")
	if !IsBackgroundProcess() {
		t.Error("expected IsBackgroundProcess to be true when env is 1")
	}
}

func TestDaemonLogPath(t *testing.T) {
	overrideHome(t) // Helper from state_test.go

	path, err := daemonLogPath()
	if err != nil {
		t.Fatalf("daemonLogPath error: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".baleen", "daemon.log")
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestLaunchBackground(t *testing.T) {
	overrideHome(t)

	err := LaunchBackground("test-token", "test-name")
	if err != nil {
		t.Fatalf("LaunchBackground failed: %v", err)
	}

	// LaunchBackground should create the log file.
	path, _ := daemonLogPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("LaunchBackground should create the daemon log file")
	}
}
