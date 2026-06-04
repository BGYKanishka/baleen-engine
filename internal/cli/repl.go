package cli

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/docker"
	"github.com/BGYKanishka/baleen-engine/internal/ledger"
	"github.com/BGYKanishka/baleen-engine/internal/network"
	"github.com/BGYKanishka/baleen-engine/internal/transfer"
	"github.com/chzyer/readline"
)

// EngineContext holds all the dependencies the CLI needs to execute commands
type EngineContext struct {
	NodeName       string
	TempDir        string
	ActualPort     int
	PeerRegistry   *network.PeerRegistry
	EngineLedger   *ledger.Ledger
	TLSConfig      *tls.Config
	ApprovalChan   chan transfer.ApprovalRequest
	DownloadedChan chan string
}

func Start(ctx EngineContext) {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "baleen> ",
		HistoryFile:     filepath.Join(ctx.TempDir, "baleen_history.tmp"),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	inputChan := make(chan string)

	// Feed readline input into our master channel
	go func() {
		for {
			line, err := rl.Readline()
			if err != nil {
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
	fmt.Println("  gc <all|old>                        - Run garbage collection on the transfer ledger")
	fmt.Println("  prune                               - Clean up old docker images")
	fmt.Println("  exit                                - Shut down engine")

	// The Master Event Loop
	for {
		select {
		case req := <-ctx.ApprovalChan:
			pendingReq = &req
			mbSize := req.Req.Size / 1024 / 1024
			fmt.Printf("\n\nINCOMING REQUEST: '%s' (%d MB) from %s\n", req.Req.ImageName, mbSize, req.Req.Author)
			rl.SetPrompt("Accept transfer? (y/n): ")
			rl.Refresh()

		case receivedPath := <-ctx.DownloadedChan:
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

			if pendingReq != nil {
				rl.SetPrompt("")
				rl.Refresh()

				if strings.ToLower(input) == "y" {
					pendingReq.Response <- true
				} else {
					pendingReq.Response <- false
				}
				pendingReq = nil

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
				handlePush(parts, rl, inputChan, ctx)
			case "peers":
				handlePeers(ctx.PeerRegistry)
			case "history":
				handleHistory(ctx.EngineLedger)
			case "gc":
				handleGC(parts, ctx)
			case "prune":
				handlePrune()
			case "exit":
				fmt.Println("\nShutting down Baleen Engine...")
				os.Exit(0)
			default:
				fmt.Println("Unknown command. Try 'push <NODE_NAME> <IMAGE>' or 'exit'")
			}
		}
	}
}

// Extracted command handlers to keep the main loop clean:
func handlePush(parts []string, rl *readline.Instance, inputChan chan string, ctx EngineContext) {
	if len(parts) < 3 {
		fmt.Println(" Usage: push <NODE_NAME_OR_IP:PORT> <IMAGE_NAME> [OPTIONAL: PATH_TO_DOCKERFILE]")
		return
	}
	targetStr := parts[1]
	targetImage := parts[2]
	buildContext := "."
	if len(parts) >= 4 {
		buildContext = parts[3]
	}
	// hybride dns resolver
	peers := ctx.PeerRegistry.GetAllPeers()
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
		ExportDir:      ctx.TempDir,
		BuildContext:   buildContext,
		ForceRawExport: false,
	}

	exportedFilePath, finalArch, err := docker.ExportImage(configReq)

	if err != nil && err.Error() == "ERR_NO_DOCKERFILE" {
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

	if err != nil {
		fmt.Printf("Export failed: %v\n", err)
		return
	}

	fmt.Printf("Streaming image to disk at: %s\n", exportedFilePath)
	hash, _ := ledger.GenerateHash(exportedFilePath)
	commit := ledger.Commit{
		Hash:      hash,
		Image:     targetImage,
		Author:    ctx.NodeName,
		Timestamp: time.Now().Format(time.RFC3339),
		Direction: "Exported",
		Status:    "Completed",
	}
	ctx.EngineLedger.RecordCommit(commit)
	fmt.Println("Commit successfully written to local Ledger!")

	err = transfer.PushImage(targetIP, targetPort, exportedFilePath, targetImage, hash, ctx.NodeName, finalArch, ctx.TLSConfig)
	if err != nil {
		fmt.Println("Push failed:", err)
	}
}

func handlePeers(registry *network.PeerRegistry) {
	peers := registry.GetAllPeers()
	if len(peers) == 0 {
		fmt.Println("No other peers found on the network yet.")
		return
	}
	fmt.Println("\nActive Baleen Nodes:")
	fmt.Println("--------------------------------------------------")
	fmt.Printf("%-20s | %-20s\n", "NODE NAME", "ADDRESS (IP:PORT)")
	fmt.Println("--------------------------------------------------")
	for name, addr := range peers {
		fmt.Printf("%-20s | %-20s\n", name, addr)
	}
	fmt.Println("--------------------------------------------------")
}

func handleHistory(engineLedger *ledger.Ledger) {
	historyList, err := engineLedger.GetHistory()
	if err != nil {
		fmt.Println("Failed to read ledger:", err)
		return
	}
	if len(historyList) == 0 {
		fmt.Println("\nThe ledger is empty. No transfers recorded yet.")
		return
	}
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
func handleGC(parts []string, ctx EngineContext) {
	engineLedger := ctx.EngineLedger
	if len(parts) < 2 {
		fmt.Println(" Usage: gc <all|old|rm> [args]")
		fmt.Println("   all     : Wipes the entire transfer ledger history")
		fmt.Println("   old [N] : Removes entries older than 7 days (or N days)")
		fmt.Println("   rm <id> : Removes a specific commit by its short hash")
		return
	}

	switch parts[1] {
	case "all":
		err := engineLedger.ClearAllHistory()
		if err != nil {
			fmt.Printf("Failed to clear ledger: %v\n", err)
			return
		}

		entries, err := os.ReadDir(ctx.TempDir)
		if err == nil {
			freedSpace := int64(0)
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".tar") {
					filePath := filepath.Join(ctx.TempDir, entry.Name())
					if info, err := os.Stat(filePath); err == nil {
						freedSpace += info.Size()
					}
					os.RemoveAll(filePath)
				}
			}
			fmt.Printf("Ledger wiped and %d MB of physical cache deleted.\n", freedSpace/1024/1024)
		} else {
			fmt.Println("Ledger history wiped (cache folder was already empty).")
		}
	case "old":
		days := 7
		if len(parts) >= 3 {
			parsedDays, err := strconv.Atoi(parts[2])
			if err != nil || parsedDays < 0 {
				fmt.Println("Invalid timeline. Please provide a positive number of days.")
				return
			}
			days = parsedDays
		}
		cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
		count, err := engineLedger.PruneHistoryOlderThan(cutoff)
		if err != nil {
			fmt.Printf("Failed to prune ledger: %v\n", err)
		} else {
			fmt.Printf("Ledger GC complete! Removed %d entries older than %d days.\n", count, days)
		}
	case "rm":
		if len(parts) < 3 {
			fmt.Println(" Usage: gc rm <short_hash>")
			return
		}
		targetHash := parts[2]
		err := engineLedger.DeleteCommit(targetHash)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Printf("Commit '%s' successfully deleted from the ledger.\n", targetHash)
		}
	default:
		fmt.Println("Unknown gc option. Use 'all', 'old', or 'rm'.")
	}
}

func handlePrune() {
	fmt.Println("Sweeping up old dangling Docker images...")
	pruneCmd := exec.Command("docker", "image", "prune", "-f")
	output, err := pruneCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Failed to run cleanup: %v\n", err)
		return
	}
	outputStr := string(output)
	fmt.Printf("\n%s\n", strings.TrimSpace(outputStr))
	if strings.Contains(outputStr, "Total reclaimed space: 0B") {
		fmt.Println("Note: No space was freed. Your old images are currently being used by active or stopped containers.")
	} else {
		fmt.Println("Cleanup complete.")
	}
}
