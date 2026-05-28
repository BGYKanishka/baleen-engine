package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/config"
	"github.com/BGYKanishka/baleen-engine/internal/docker"
	"github.com/BGYKanishka/baleen-engine/internal/ledger"
	"github.com/BGYKanishka/baleen-engine/internal/network"
	"github.com/BGYKanishka/baleen-engine/internal/transfer"
	"github.com/chzyer/readline"
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
	// Initialize the persistent Ledger DB
	engineLedger, err := ledger.NewLedger(dbPath)
	if err != nil {
		panic(fmt.Errorf("failed to open ledger database: %w", err))
	}
	defer engineLedger.Close()

	fmt.Println("Generating ephemeral TLS certificates for secure transfers...")
	tlsConfig, err := network.GenerateEphemeralTLS()
	if err != nil {
		panic(fmt.Errorf("failed to generate TLS config: %w", err))
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

	go transfer.StartReceiver(*port, incomingDir, approvalChan, downloadedChan, engineLedger, tlsConfig)

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

	// Initialize readline for interactive CLI with history support
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "baleen> ",
		HistoryFile:     filepath.Join(tempDir, "baleen_history.tmp"),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	// Feed readline input into our master channel
	go func() {
		for {
			line, err := rl.Readline()
			if err != nil { // Handles Ctrl+C or Ctrl+D
				if err == readline.ErrInterrupt {
					continue
				}
				os.Exit(0)
			}
			inputChan <- line
		}
	}()

	var pendingReq *transfer.ApprovalRequest

	fmt.Println("\nBaleen Engine is now online!")
	fmt.Println("Commands:")
	fmt.Println("  push <NODE_NAME_OR_IP:PORT> <IMAGE> - Send a specific Docker image to target")
	fmt.Println("  peers                               - Show active nodes on network")
	fmt.Println("  history                             - View the transfer ledger")
	fmt.Println("  exit                                - Shut down engine")

	// The Master Event Loop
	for {
		select {
		case req := <-approvalChan:
			pendingReq = &req
			mbSize := req.Req.Size / 1024 / 1024
			fmt.Printf("\n\nINCOMING REQUEST: '%s' (%d MB) from %s\n", req.Req.ImageName, mbSize, req.Req.Author)
			rl.SetPrompt("Accept transfer? (y/n): ")
			rl.Refresh()

		case receivedPath := <-downloadedChan:
			fmt.Println("\nUnpacking and loading image into Docker Daemon...")
			err := docker.LoadImage(receivedPath)
			if err != nil {
				fmt.Println("Failed to load image into Docker:", err)
			} else {
				fmt.Println("Image successfully loaded! (Type 'docker images' in another terminal to verify)")
			}
			rl.Refresh()

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
				rl.SetPrompt("baleen> ")
				rl.Refresh()
				continue
			}

			parts := strings.Split(input, " ")
			if len(parts) == 0 || parts[0] == "" {
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
						ForceRawExport: false,
					}

					exportedFilePath, finalArch, err := docker.ExportImage(configReq)

					if err != nil {
						if err.Error() == "ERR_NO_DOCKERFILE" {
							fmt.Printf("\n No Dockerfile found at '%s'. Cannot autonomously cross-compile.\n", buildContext)
							rl.SetPrompt("Enter path to your project folder, or type 'n' to send as-is: ")
							rl.Refresh()

							// wait for the user to type into the channel
							response := <-inputChan
							response = strings.TrimSpace(response)

							// Restore the prompt
							rl.SetPrompt("baleen> ")
							rl.Refresh()

							if strings.ToLower(response) == "n" {
								fmt.Println("\nInitiating Raw Transfer Mode (Emulation Fallback)...")
								configReq.ForceRawExport = true
								exportedFilePath, finalArch, err = docker.ExportImage(configReq)
							} else {
								fmt.Printf("\nRetrying cross-compilation with build context: %s\n", response)
								configReq.BuildContext = response
								exportedFilePath, finalArch, err = docker.ExportImage(configReq)
							}
						}
					}

					if err != nil {
						fmt.Printf("Export failed: %v\n", err)
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
					engineLedger.RecordCommit(commit)
					fmt.Println("Commit successfully written to local Ledger!")

					err = transfer.PushImage(targetIP, targetPort, exportedFilePath, targetImage, hash, *nodeName, finalArch, tlsConfig)
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
			case "history":
				historyList, err := engineLedger.GetHistory()
				if err != nil {
					fmt.Println("Failed to read ledger:", err)
					continue
				}

				if len(historyList) == 0 {
					fmt.Println("\nThe ledger is empty. No transfers recorded yet.")
				} else {
					fmt.Println("\nBaleen Transfer Ledger:")
					fmt.Println("--------------------------------------------------------------------------------------------------")
					fmt.Printf("%-20s | %-12s | %-10s | %-30s | %-10s\n", "DATE", "DIRECTION", "STATUS", "IMAGE", "HASH")
					fmt.Println("--------------------------------------------------------------------------------------------------")
					for _, c := range historyList {
						shortHash := c.Hash
						if len(shortHash) > 8 {
							shortHash = shortHash[:8]
						}

						// Format the timestamp
						parsedTime, _ := time.Parse(time.RFC3339, c.Timestamp)
						displayTime := parsedTime.Format("Jan 02 15:04:05")

						displayImage := c.Image
						if len(displayImage) > 30 {
							displayImage = displayImage[:27] + "..."
						}

						fmt.Printf("%-20s | %-12s | %-10s | %-30s | %-10s\n", displayTime, c.Direction, c.Status, displayImage, shortHash)
					}
					fmt.Println("--------------------------------------------------------------------------------------------------")
				}
			case "prune":
				fmt.Println("Sweeping up old dangling Docker images...")

				pruneCmd := exec.Command("docker", "image", "prune", "-f")
				output, err := pruneCmd.CombinedOutput()

				if err != nil {
					fmt.Printf("Failed to run cleanup: %v\n", err)
				} else {
					outputStr := string(output)
					fmt.Printf("\n%s\n", strings.TrimSpace(outputStr))

					if strings.Contains(outputStr, "Total reclaimed space: 0B") {
						fmt.Println("Note: No space was freed. Your old images are currently being used by active or stopped containers.")
					} else {
						fmt.Println("Cleanup complete.")
					}
				}

			case "exit":
				fmt.Println("\nShutting down Baleen Engine...")
				os.Exit(0)
			default:
				fmt.Println("Unknown command. Try 'push <NODE_NAME> <IMAGE>' or 'exit'")
			}
		}
	}
}
