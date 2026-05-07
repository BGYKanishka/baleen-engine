package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/config"
	"github.com/BGYKanishka/baleen-engine/internal/docker"
	"github.com/BGYKanishka/baleen-engine/internal/ledger"
	"github.com/BGYKanishka/baleen-engine/internal/network"
)

func main() {
	fmt.Println("Starting Baleen Engine (Go Edition)...")

	// 1. Setup Environment
	tempDir, dbPath, err := config.SetupBaleenDirectory()
	if err != nil {
		panic(fmt.Errorf("failed to setup directories: %w", err))
	}

	// 2. Start Network Broadcaster
	nodeName := "Kanishka-MacBook"
	go network.StartBroadcaster(nodeName, 8080)

	// 3. Export Image
	targetImage := "alpine:latest"
	fmt.Printf("Preparing to export '%s'...\n", targetImage)

	exportedFilePath, err := docker.ExportImage(targetImage, tempDir)
	if err != nil {
		fmt.Printf("Export failed: %v\n", err)
	} else {
		fmt.Printf("Streaming image to disk at: %s\n", exportedFilePath)
		fmt.Println("Export complete!")

		// 4. Hash and Commit
		hash, _ := ledger.GenerateHash(exportedFilePath)
		commit := ledger.Commit{
			Hash:      hash,
			Image:     targetImage,
			Author:    nodeName,
			Timestamp: time.Now().Format(time.RFC3339),
			Direction: "Exported",
			Status:    "Completed",
		}
		ledger.RecordCommit(dbPath, commit)
		fmt.Println("Commit successfully written to local Ledger!")
	}

	// 5. Keep Alive
	fmt.Println("\nBaleen Engine is now online and listening for peers.")
	fmt.Println("Press Ctrl+C to shut down.")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	fmt.Println("\nShutting down Baleen Engine...")
}