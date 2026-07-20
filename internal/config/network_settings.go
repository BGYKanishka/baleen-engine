package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// holds the user-configurable network feature flags.
type NetworkSettings struct {
	MDNSDiscovery     bool `json:"mdns_discovery"`
	BroadcastPresence bool `json:"broadcast_presence"`
}

// returns the out-of-the-box defaults (both on).
func defaultNetworkSettings() NetworkSettings {
	return NetworkSettings{
		MDNSDiscovery:     true,
		BroadcastPresence: true,
	}
}

func networkSettingsPath() (string, error) {
	baleenRoot, err := BaleenDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baleenRoot, "network_settings.json"), nil
}

// reads the persisted settings file
// If the file does not exist or is invalid, returns the default settings.
func LoadNetworkSettings() NetworkSettings {
	path, err := networkSettingsPath()
	if err != nil {
		return defaultNetworkSettings()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultNetworkSettings()
	}
	var s NetworkSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return defaultNetworkSettings()
	}
	return s
}

// writes the settings to ~/.baleen/network_settings.json.
func SaveNetworkSettings(s NetworkSettings) error {
	path, err := networkSettingsPath()
	if err != nil {
		return err
	}
	// Ensure the directory exists (it should already, but be safe).
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
