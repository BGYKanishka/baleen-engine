package config

import (
	"os"
	"path/filepath"
)

// creates the folder and returns paths
func SetupBaleenDirectory() (string, string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}

	baleenRoot := filepath.Join(homeDir, ".baleen")
	baleenTempDir := filepath.Join(baleenRoot, "temp")
	dbPath := filepath.Join(baleenRoot, "baleen.db")

	err = os.MkdirAll(baleenTempDir, 0755)
	if err != nil {
		return "", "", err
	}

	return baleenTempDir, dbPath, nil
}