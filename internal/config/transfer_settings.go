package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// holds the user-configurable transfer settings.
type TransferSettings struct {
	AutoApprove  bool `json:"auto_approve"`
	MaxBandwidth int  `json:"max_bandwidth"` // in MB/s
}

// returns the out-of-the-box defaults.
func defaultTransferSettings() TransferSettings {
	return TransferSettings{
		AutoApprove:  false,
		MaxBandwidth: 50,
	}
}

func transferSettingsPath() (string, error) {
	baleenRoot, err := BaleenDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baleenRoot, "transfer_settings.json"), nil
}

// reads the persisted settings file
// If the file does not exist or is invalid, returns the default settings.
func LoadTransferSettings() TransferSettings {
	path, err := transferSettingsPath()
	if err != nil {
		return defaultTransferSettings()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultTransferSettings()
	}
	var s TransferSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return defaultTransferSettings()
	}
	return s
}

// writes the settings to ~/.baleen/transfer_settings.json.
func SaveTransferSettings(s TransferSettings) error {
	path, err := transferSettingsPath()
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
