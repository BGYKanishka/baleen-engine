package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/client"
	"go.etcd.io/bbolt"
)

// Commit represents one record in the local ledger.
type Commit struct {
	Hash      string `json:"hash"`
	Image     string `json:"image"`
	Author    string `json:"author"`
	Timestamp string `json:"timestamp"`
	Direction string `json:"direction"`
	Status    string `json:"status"`
}

func main() {
	fmt.Println("Starting Baleen Engine (Go Edition)...")

	// Create Baleen temp directory and database path.
	tempDir, dbPath, err := setupBaleenDirectory()
	if err != nil {
		panic(fmt.Errorf("failed to setup directories: %w", err))
	}

	// Connect to the local docker daemon.
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	defer cli.Close()

	// Docker image to export.
	targetImage := "alpine:latest"
	fmt.Printf("Preparing to export '%s'...\n", targetImage)

	// Export the Docker image.
	exportedFilePath, err := exportImage(cli, targetImage, tempDir)
	if err != nil {
		fmt.Printf("Export failed: %v\n", err)
		return
	}

	fmt.Println("Export complete!")

	// Generate SHA-256 hash for the exported tar file.
	fmt.Println("Calculating SHA-256 fingerprint...")
	hash, err := generateHash(exportedFilePath)
	if err != nil {
		fmt.Printf("Hashing failed: %v\n", err)
		return
	}

	fmt.Printf("Hash: %s\n", hash)

	// Create a commit record for the local ledger.
	commit := Commit{
		Hash:      hash,
		Image:     targetImage,
		Author:    "Local-User",
		Timestamp: time.Now().Format(time.RFC3339),
		Direction: "Exported",
		Status:    "Completed",
	}

	// Save commit metadata in the local ledger.
	err = recordCommit(dbPath, commit)
	if err != nil {
		fmt.Printf("Failed to update ledger: %v\n", err)
		return
	}

	fmt.Println("Commit successfully written to local ledger!")
}

// creates ~/.baleen/temp and returns its path with the database path.
func setupBaleenDirectory() (string, string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}

	baleenRoot := filepath.Join(homeDir, ".baleen")
	baleenTempDir := filepath.Join(baleenRoot, "temp")
	dbPath := filepath.Join(baleenRoot, "baleen.db")

	if err := os.MkdirAll(baleenTempDir, 0755); err != nil {
		return "", "", err
	}

	return baleenTempDir, dbPath, nil
}

// saves a Docker image as a tar file and returns the saved file path.
func exportImage(cli *client.Client, imageName string, exportDir string) (string, error) {
	ctx := context.Background()

	// Request the image tar stream from Docker.
	imageStream, err := cli.ImageSave(ctx, []string{imageName})
	if err != nil {
		return "", err
	}
	defer imageStream.Close()

	// Convert image name into a safe file name.
	safeFilename := strings.ReplaceAll(imageName, ":", "_") + ".tar"
	targetPath := filepath.Join(exportDir, safeFilename)

	// Create the output tar file.
	outFile, err := os.Create(targetPath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	fmt.Printf("Streaming image to disk at: %s\n", targetPath)

	// Copy Docker's image stream directly to disk.
	if _, err := io.Copy(outFile, imageStream); err != nil {
		return "", err
	}

	return targetPath, nil
}

// creates SHA-256 hash from the exported tar file.
func generateHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()

	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// saves commit metadata in the local bbolt.
func recordCommit(dbPath string, commit Commit) error {
	// Open the local db file.
	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		return err
	}
	defer db.Close()

	// Start a read-write transaction.
	return db.Update(func(tx *bbolt.Tx) error {
		// Create Ledger bucket if it not exist.
		bucket, err := tx.CreateBucketIfNotExists([]byte("Ledger"))
		if err != nil {
			return err
		}

		// Convert commit data into JSON.
		commitJSON, err := json.Marshal(commit)
		if err != nil {
			return err
		}

		// Save the commit using the hash as the key.
		return bucket.Put([]byte(commit.Hash), commitJSON)
	})
}