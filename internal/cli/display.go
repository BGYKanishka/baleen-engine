package cli

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/ledger"
	"github.com/BGYKanishka/baleen-engine/internal/network"
)

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
		parsedTime, _ := time.Parse(time.RFC3339, c.Timestamp)
		displayTime := parsedTime.Format("Jan 02 15:04:05")

		displayImage := c.Image
		if len(displayImage) > 30 {
			displayImage = displayImage[:27] + "..."
		}

		fmt.Printf("%-20s | %-12s | %-10s | %-30s | %-10s\n",
			displayTime, c.Direction, c.Status, displayImage, shortHash)
	}
	fmt.Println("--------------------------------------------------------------------------------------------------")
}

func handlePrune() {
	fmt.Println("Sweeping up old dangling Docker images...")

	output, err := exec.Command("docker", "image", "prune", "-f").CombinedOutput()
	if err != nil {
		fmt.Printf("Failed to run cleanup: %v\n", err)
		return
	}

	outputStr := strings.TrimSpace(string(output))
	fmt.Printf("\n%s\n", outputStr)

	if strings.Contains(outputStr, "Total reclaimed space: 0B") {
		fmt.Println("Note: No space was freed. Your old images are currently being used by active or stopped containers.")
	} else {
		fmt.Println("Cleanup complete.")
	}
}
