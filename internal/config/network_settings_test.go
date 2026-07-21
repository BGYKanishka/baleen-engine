package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultNetworkSettings(t *testing.T) {
	s := DefaultNetworkSettings()
	if !s.MDNSDiscovery || !s.BroadcastPresence {
		t.Errorf("Expected defaults to be true, got MDNS=%v Broadcast=%v", s.MDNSDiscovery, s.BroadcastPresence)
	}
}

func TestSaveAndLoadNetworkSettings(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	settings := NetworkSettings{
		MDNSDiscovery:     false,
		BroadcastPresence: true,
	}

	err := SaveNetworkSettings(settings)
	if err != nil {
		t.Fatalf("Failed to save network settings: %v", err)
	}

	loaded := LoadNetworkSettings()
	if loaded.MDNSDiscovery != false || loaded.BroadcastPresence != true {
		t.Errorf("Expected MDNS=false, Broadcast=true; got MDNS=%v, Broadcast=%v", loaded.MDNSDiscovery, loaded.BroadcastPresence)
	}
}

func TestLoadNetworkSettings_NotFound(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Should return default
	loaded := LoadNetworkSettings()
	if !loaded.MDNSDiscovery || !loaded.BroadcastPresence {
		t.Errorf("Expected default (true, true) when not found, got (%v, %v)", loaded.MDNSDiscovery, loaded.BroadcastPresence)
	}
}

func TestLoadNetworkSettings_InvalidJSON(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Write invalid json
	dir := filepath.Join(tempHome, ".baleen")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "network_settings.json"), []byte("{invalid json}"), 0644)

	loaded := LoadNetworkSettings()
	if !loaded.MDNSDiscovery || !loaded.BroadcastPresence {
		t.Errorf("Expected default (true, true) when JSON is invalid, got (%v, %v)", loaded.MDNSDiscovery, loaded.BroadcastPresence)
	}
}
