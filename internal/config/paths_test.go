package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBaleenDir(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	expected := filepath.Join(tempHome, ".baleen")
	dir, err := BaleenDir()
	if err != nil {
		t.Fatalf("BaleenDir failed: %v", err)
	}
	if dir != expected {
		t.Errorf("Expected %q, got %q", expected, dir)
	}
}

func TestServiceStatePath(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	expected := filepath.Join(tempHome, ".baleen", "service.json")
	path, err := ServiceStatePath()
	if err != nil {
		t.Fatalf("ServiceStatePath failed: %v", err)
	}
	if path != expected {
		t.Errorf("Expected %q, got %q", expected, path)
	}
}

func TestSetupBaleenDirectory(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	tempDir, incomingDir, dbPath, err := SetupBaleenDirectory()
	if err != nil {
		t.Fatalf("SetupBaleenDirectory failed: %v", err)
	}

	baleenRoot := filepath.Join(tempHome, ".baleen")
	if tempDir != filepath.Join(baleenRoot, "temp") {
		t.Errorf("Unexpected tempDir: %s", tempDir)
	}
	if incomingDir != filepath.Join(baleenRoot, "incoming") {
		t.Errorf("Unexpected incomingDir: %s", incomingDir)
	}
	if dbPath != filepath.Join(baleenRoot, "baleen.db") {
		t.Errorf("Unexpected dbPath: %s", dbPath)
	}

	// Verify directories were actually created
	for _, dir := range []string{tempDir, incomingDir} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("Directory %s was not created: %v", dir, err)
		} else if !info.IsDir() {
			t.Errorf("Path %s exists but is not a directory", dir)
		}
	}
}
