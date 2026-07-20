package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateNodeName(t *testing.T) {
	name1 := GenerateNodeName()
	name2 := GenerateNodeName()

	if name1 == "" || name2 == "" {
		t.Error("GenerateNodeName returned empty string")
	}

	if name1 == name2 {
		t.Logf("Warning: GenerateNodeName returned the same name twice (%s), which is unlikely but possible", name1)
	}

	parts := strings.Split(name1, "-")
	if len(parts) != 3 {
		t.Errorf("Expected node name to have 3 parts separated by '-', got %v", parts)
	}
}

func TestSaveAndLoadNodeName(t *testing.T) {
	// Mock the home directory so we don't mess with real data
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Ensure .baleen directory exists as SaveNodeName expects it
	os.MkdirAll(filepath.Join(tempHome, ".baleen"), 0755)

	testName := "Aqua-Shark-42"
	
	// Test Save
	err := SaveNodeName(testName)
	if err != nil {
		t.Fatalf("Failed to save node name: %v", err)
	}

	// Test Load
	loadedName := LoadNodeName()
	if loadedName != testName {
		t.Errorf("Expected to load %q, got %q", testName, loadedName)
	}
}

func TestLoadNodeName_NotFound(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	loadedName := LoadNodeName()
	if loadedName != "" {
		t.Errorf("Expected empty string when no node name is saved, got %q", loadedName)
	}
}
