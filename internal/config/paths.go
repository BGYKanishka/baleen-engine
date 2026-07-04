package config

import (
	"os"
	"path/filepath"
)

// MetadataPortOffset is the offset added to the main port to determine the metadata server port
const MetadataPortOffset = 1

// returns the path to the running-service state file.
func ServiceStatePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".baleen", "service.json"), nil
}

// creates the folder and returns paths
func SetupBaleenDirectory() (string, string, string, string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", "", err
	}

	baleenRoot := filepath.Join(homeDir, ".baleen")
	tempDir := filepath.Join(baleenRoot, "temp")
	incomingDir := filepath.Join(baleenRoot, "incoming")
	dbPath := filepath.Join(baleenRoot, "baleen.db")
	certsDir := filepath.Join(baleenRoot, "certs")

	// Create both directories
	os.MkdirAll(tempDir, 0755)
	os.MkdirAll(incomingDir, 0755)
	os.MkdirAll(certsDir, 0755)

	return tempDir, incomingDir, dbPath, certsDir, nil
}
