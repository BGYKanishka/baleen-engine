package config

import (
	"os"
	"path/filepath"
)

// MetadataPortOffset is the offset added to the main port to determine the metadata server port
const MetadataPortOffset = 1

// BaleenDir returns the path to the user's .baleen directory.
func BaleenDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".baleen"), nil
}

// returns the path to the running-service state file.
func ServiceStatePath() (string, error) {
	baleenRoot, err := BaleenDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baleenRoot, "service.json"), nil
}

// creates the folder and returns paths
func SetupBaleenDirectory() (string, string, string, error) {
	baleenRoot, err := BaleenDir()
	if err != nil {
		return "", "", "", err
	}

	tempDir := filepath.Join(baleenRoot, "temp")
	incomingDir := filepath.Join(baleenRoot, "incoming")
	dbPath := filepath.Join(baleenRoot, "baleen.db")

	// Create directories (certs dir is no longer needed — certs are ephemeral)
	os.MkdirAll(tempDir, 0755)
	os.MkdirAll(incomingDir, 0755)

	return tempDir, incomingDir, dbPath, nil
}
