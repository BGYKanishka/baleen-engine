package config

import (
	"os"
	"path/filepath"
)

// creates the folder and returns paths
func SetupBaleenDirectory() (string, string, string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", err
	}

	baleenRoot := filepath.Join(homeDir, ".baleen")
	tempDir := filepath.Join(baleenRoot, "temp")
	incomingDir := filepath.Join(baleenRoot, "incoming")
	dbPath := filepath.Join(baleenRoot, "baleen.db")

	// Create both directories
	os.MkdirAll(tempDir, 0755)
	os.MkdirAll(incomingDir, 0755)

	return tempDir, incomingDir, dbPath, nil
}
