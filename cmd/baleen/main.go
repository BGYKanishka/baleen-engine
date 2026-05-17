package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
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

	peerRegistry := network.NewPeerRegistry()
	go network.DiscoverPeers(*nodeName, peerRegistry)

	// background Checker!
	go peerRegistry.StartHealthChecker()

	// Create channels for our inputs
	inputChan := make(chan string)
	approvalChan := make(chan transfer.ApprovalRequest)
	downloadedChan := make(chan string)

	go transfer.StartReceiver(*port, incomingDir, approvalChan, downloadedChan)

	go func() {
		metadataPort := *port + 1
		http.HandleFunc("/architecture", func(w http.ResponseWriter, r *http.Request) {
			arch := "linux/" + runtime.GOARCH
			w.Write([]byte(arch))
		})

		err := http.ListenAndServe(fmt.Sprintf(":%d", metadataPort), nil)
		if err != nil && err != http.ErrServerClosed {
			fmt.Println("Metadata server error:", err)
		}
	}()

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			inputChan <- scanner.Text()
		}
	}()

	var pendingReq *transfer.ApprovalRequest

	fmt.Println("\nBaleen Engine is now online!")
	fmt.Println("Commands:")
	fmt.Println("  push <NODE_NAME_OR_IP:PORT> <IMAGE> - Send a specific Docker image to target")
	fmt.Println("  peers                               - Show active nodes on network")
	fmt.Println("  exit                                - Shut down engine")
	fmt.Print("\nbaleen> ")

	// The Master Event Loop
	for {
		select {
		case req := <-approvalChan:
			pendingReq = &req
			mbSize := req.Req.Size / 1024 / 1024
			fmt.Printf("\n\nINCOMING REQUEST: '%s' (%d MB) from %s\n", req.Req.ImageName, mbSize, req.Req.Author)
			fmt.Print("Accept transfer? (y/n): ")

		case receivedPath := <-downloadedChan:
			fmt.Println("\nUnpacking and loading image into Docker Daemon...")
			err := docker.LoadImage(receivedPath)
			if err != nil {
				fmt.Println("Failed to load image into Docker:", err)
			} else {
				fmt.Println("Image successfully loaded! (Type 'docker images' in another terminal to verify)")
			}
			fmt.Print("\nbaleen> ")

		case input := <-inputChan:
			input = strings.TrimSpace(input)

			// waiting for user approval
			if pendingReq != nil {
				if strings.ToLower(input) == "y" {
					pendingReq.Response <- true
				} else {
					pendingReq.Response <- false
				}
				pendingReq = nil
				time.Sleep(100 * time.Millisecond)
				fmt.Print("\nbaleen> ")
				continue
			}

			parts := strings.Split(input, " ")
			if len(parts) == 0 || parts[0] == "" {
				fmt.Print("baleen> ")
				continue
			}

			switch parts[0] {
			case "push":
				if len(parts) < 3 {
					fmt.Println(" Usage: push <NODE_NAME_OR_IP:PORT> <IMAGE_NAME> [OPTIONAL: PATH_TO_DOCKERFILE]")
				} else {
					targetStr := parts[1]
					targetImage := parts[2]

					buildContext := "."
					if len(parts) >= 4 {
						buildContext = parts[3]
					}

					// hybride dns resolver
					peers := peerRegistry.GetAllPeers()
					if resolvedIP, exists := peers[targetStr]; exists {
						fmt.Printf("\nResolved Node '%s' to %s\n", targetStr, resolvedIP)
						targetStr = resolvedIP
					}

					targetIP := targetStr
					targetPort := 8080

					if strings.Contains(targetStr, ":") {
						split := strings.Split(targetStr, ":")
						targetIP = split[0]
						fmt.Sscanf(split[1], "%d", &targetPort)
					}
	
					fmt.Printf("\nPinging %s to detect architecture...\n", targetIP)
					targetArch := "linux/amd64" // Fallback default

					metadataURL := fmt.Sprintf("http://%s:%d/architecture", targetIP, targetPort+1)

					// Simple HTTP client with timeout to get target architecture
					client := http.Client{Timeout: 2 * time.Second}
					resp, err := client.Get(metadataURL)

					if err == nil {
						bodyBytes, _ := io.ReadAll(resp.Body)
						resp.Body.Close()
						targetArch = strings.TrimSpace(string(bodyBytes))
						fmt.Printf("Target architecture detected: %s\n", targetArch)
					} else {
						fmt.Printf("Could not reach pre-flight server at %s. Falling back to %s\n", targetIP, targetArch)
					}

					fmt.Printf("Preparing to export '%s'...\n", targetImage)

					// Pass the extracted buildContext down into the engine
					configReq := docker.PreflightConfig{
						ImageName:      targetImage,
						ExpectedTarget: targetArch,
						ExportDir:      tempDir,
						BuildContext:   buildContext,
					}

					exportedFilePath, err := docker.ExportImage(configReq)
					if err != nil {
						fmt.Printf("Export failed: %v\n", err)
						fmt.Print("\nbaleen> ")
						continue
					}

					fmt.Printf("Streaming image to disk at: %s\n", exportedFilePath)

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

					// Trigger the network transfer
					err = transfer.PushImage(targetIP, targetPort, exportedFilePath, targetImage, hash, *nodeName)
					if err != nil {
						fmt.Println("Push failed:", err)
					}
				}
			case "peers":
				peers := peerRegistry.GetAllPeers()
				if len(peers) == 0 {
					fmt.Println("No other peers found on the network yet.")
				} else {
					fmt.Println("\nActive Baleen Nodes:")
					fmt.Println("--------------------------------------------------")
					fmt.Printf("%-20s | %-20s\n", "NODE NAME", "ADDRESS (IP:PORT)")
					fmt.Println("--------------------------------------------------")
					for name, addr := range peers {
						fmt.Printf("%-20s | %-20s\n", name, addr)
					}
					fmt.Println("--------------------------------------------------")
				}

			case "exit":
				fmt.Println("\nShutting down Baleen Engine...")
				os.Exit(0)
			default:
				fmt.Println("Unknown command. Try 'push <NODE_NAME> <IMAGE>' or 'exit'")
			}
			fmt.Print("\nbaleen> ")
		}
	}
}
