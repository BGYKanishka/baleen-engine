package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/config"
	"github.com/BGYKanishka/baleen-engine/internal/docker"
	"github.com/BGYKanishka/baleen-engine/internal/ledger"
	"github.com/BGYKanishka/baleen-engine/internal/network"
	"github.com/BGYKanishka/baleen-engine/internal/transfer"
)

func main() {
	nodeName := flag.String("name", "Kanishka-MacBook", "Name of the Baleen Node")
	port := flag.Int("port", 8080, "Port for the Baleen Node")
	flag.Parse()

	fmt.Printf("Starting Baleen Engine as '%s' on Port %d...\n", *nodeName, *port)

	// Setup Environment
	tempDir, incomingDir, dbPath, err := config.SetupBaleenDirectory()
	if err != nil {
		panic(fmt.Errorf("failed to setup directories: %w", err))
	}

	// Start Network Broadcaster & Listener
	go network.StartBroadcaster(*nodeName, *port)
	go network.DiscoverPeers(*nodeName)
	go transfer.StartReceiver(*port, incomingDir)

	// Export Image
	targetImage := "alpine:latest"
	fmt.Printf("Preparing to export '%s'...\n", targetImage)

	exportedFilePath, err := docker.ExportImage(targetImage, tempDir)
	if err != nil {
		fmt.Printf("Export failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Streaming image to disk at: %s\n", exportedFilePath)
	fmt.Println("Export complete!")

	// Hash and Commit
	hash, _ := ledger.GenerateHash(exportedFilePath)
	commit := ledger.Commit{
		Hash:      hash,
		Image:     targetImage,
		Author:    *nodeName,
		Timestamp: time.Now().Format(time.RFC3339),
		Direction: "Exported",
		Status:    "Completed",
	}
	ledger.RecordCommit(dbPath, commit)
	fmt.Println("Commit successfully written to local Ledger!")

	// Interactive CLI Loop
	fmt.Println("\nBaleen Engine is now online!")
	fmt.Println("Commands:")
	fmt.Println("  push <IP>   - Send alpine:latest to target IP")
	fmt.Println("  exit        - Shut down engine")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\nbaleen> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		parts := strings.Split(input, " ")

		if len(parts) == 0 || parts[0] == "" {
			continue
		}

		switch parts[0] {
		case "push":
			if len(parts) < 2 {
				fmt.Println("⚠️ Usage: push <IP> or push <IP>:<PORT>")
				continue
			}

			targetStr := parts[1]
			targetIP := targetStr
			targetPort := 8080

			// Check if the user specified a custom port
			if strings.Contains(targetStr, ":") {
				split := strings.Split(targetStr, ":")
				targetIP = split[0]
				fmt.Sscanf(split[1], "%d", &targetPort)
			}

			err := transfer.PushImage(targetIP, targetPort, exportedFilePath, targetImage, hash, *nodeName)
			if err != nil {
				fmt.Println("Push failed:", err)
			}
		case "exit":
			fmt.Println("\nShutting down Baleen Engine...")
			os.Exit(0)
		default:
			fmt.Println("Unknown command. Try 'push <IP>' or 'exit'")
		}
	}
}
