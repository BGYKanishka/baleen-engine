package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/client"
)

func main() {
	fmt.Println("Starting Baleen Engine (Go Edition)...")

	// Create Baleen temp directory.
	baleenDir, err := setupBaleenDirectory()
	if err != nil {
		panic(fmt.Errorf("failed to setup directories: %w", err))
	}

	// Connect to the local docker daemon
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	defer func(cli *client.Client) {
		err := cli.Close()
		if err != nil {

		}
	}(cli)

	// Docker image to export.
	targetImage := "alpine:latest"
	fmt.Printf("Preparing to export '%s'...\n", targetImage)

	if err := exportImage(cli, targetImage, baleenDir); err != nil {
		fmt.Printf("Export failed: %v\n", err)
		return
	}

	fmt.Println("Export complete!")
}

// creates ~/.baleen/temp and returns its path.
func setupBaleenDirectory() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	baleenTempDir := filepath.Join(homeDir, ".baleen", "temp")

	if err := os.MkdirAll(baleenTempDir, 0755); err != nil {
		return "", err
	}

	return baleenTempDir, nil
}

// saves a Docker image as a tar file
func exportImage(cli *client.Client, imageName string, exportDir string) error {
	ctx := context.Background()

	// Request the image tar stream from Docker.
	imageStream, err := cli.ImageSave(ctx, []string{imageName})
	if err != nil {
		return err
	}
	defer func(imageStream io.ReadCloser) {
		err := imageStream.Close()
		if err != nil {

		}
	}(imageStream)

	// Convert image name into a safe file name.
	safeFilename := strings.ReplaceAll(imageName, ":", "_") + ".tar"
	targetPath := filepath.Join(exportDir, safeFilename)

	// Create the output tar file.
	outFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer func(outFile *os.File) {
		err := outFile.Close()
		if err != nil {

		}
	}(outFile)

	fmt.Printf("Streaming image to disk at: %s\n", targetPath)

	// Copy Docker's image stream directly to disk.
	if _, err := io.Copy(outFile, imageStream); err != nil {
		return err
	}

	return nil
}
