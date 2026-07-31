package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/service"
	"github.com/BGYKanishka/baleen-engine/internal/transfer"
	"github.com/chzyer/readline"
)

// thin HTTP client for CLI-to-daemon communication.
type daemonClient struct {
	base  string
	token string
	http  *http.Client
}

func newDaemonClient(state service.ServiceState) *daemonClient {
	return &daemonClient{
		base:  fmt.Sprintf("http://127.0.0.1:%d", state.Port),
		token: state.Token,
		http:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *daemonClient) get(path string) (*http.Response, error) {
	req, _ := http.NewRequest(http.MethodGet, c.base+path, nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	return c.http.Do(req)
}

func (c *daemonClient) post(path string, body any) (*http.Response, error) {
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, c.base+path, bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	return c.http.Do(req)
}

// isHealthy returns true if the daemon is still reachable.
func (c *daemonClient) isHealthy() bool {
	resp, err := c.get("/api/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// starts a REPL that delegates all commands to the running
// daemon via HTTP. The daemon keeps running when the CLI exits.
func StartClientREPL(state service.ServiceState, tempDir string) {
	client := newDaemonClient(state)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "baleen> ",
		HistoryFile:     filepath.Join(tempDir, "baleen_history.tmp"),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		fmt.Printf("Failed to initialize REPL: %v\n", err)
		os.Exit(1)
	}
	defer rl.Close()

	// Channel the poll goroutine uses to surface incoming transfer requests.
	pendingChan := make(chan transfer.TransferRequest, 1)
	go pollPendingApproval(client, pendingChan)

	inputChan := make(chan string)
	syncChan := make(chan struct{})
	go feedInput(rl, inputChan, syncChan)

	printClientWelcome(state)

	var pendingApproval *transfer.TransferRequest

	for {
		select {
		case req := <-pendingChan:
			pendingApproval = &req
			mbSize := req.Size / 1024 / 1024
			fmt.Printf("\n\nINCOMING REQUEST: '%s' (%d MB) from %s\n", req.ImageName, mbSize, req.Author)
			rl.SetPrompt("Accept transfer? (y/n): ")
			rl.Refresh()

		case input := <-inputChan:
			input = strings.TrimSpace(input)

			if pendingApproval != nil {
				clientResolveApproval(input, client, rl)
				pendingApproval = nil
				syncChan <- struct{}{}
				continue
			}

			handleClientCommand(input, client, rl)
			syncChan <- struct{}{}
		}
	}
}

// feedInput reads user input from the readline instance and sends it to the input channel.
func pollPendingApproval(client *daemonClient, out chan<- transfer.TransferRequest) {
	var lastHash string
	for {
		time.Sleep(2 * time.Second)
		resp, err := client.get("/api/pending")
		if err != nil || resp.StatusCode == http.StatusNoContent {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}
		var req transfer.TransferRequest
		if err := json.NewDecoder(resp.Body).Decode(&req); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		key := req.Hash + req.ImageName
		if key != lastHash {
			lastHash = key
			select {
			case out <- req:
			default:
			}
		}
	}
}

// sends the user's y/n to /api/approve or /api/reject.
func clientResolveApproval(input string, client *daemonClient, rl *readline.Instance) {
	rl.SetPrompt("baleen> ")
	rl.Refresh()

	endpoint := "/api/reject"
	if strings.ToLower(input) == "y" {
		endpoint = "/api/approve"
	}
	resp, err := client.post(endpoint, nil)
	if err != nil {
		fmt.Printf("Failed to send decision: %v\n", err)
		return
	}
	resp.Body.Close()
	if endpoint == "/api/approve" {
		fmt.Println("Transfer approved.")
	} else {
		fmt.Println("Transfer rejected.")
	}
}

// Command dispatch
func handleClientCommand(input string, client *daemonClient, rl *readline.Instance) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "peers":
		clientHandlePeers(client)

	case "push":
		if len(parts) < 3 {
			fmt.Println("  Usage: push <NODE_NAME_OR_IP:PORT> <IMAGE_NAME>")
			return
		}
		clientHandlePush(client, parts[1], parts[2])

	case "history":
		clientHandleHistory(client)

	case "gc":
		clientHandleGC(client, parts)

	case "prune":
		clientRunPrune()

	case "clean-logs":
		clientHandleCleanLogs(client, parts)

	case "logs":
		clientHandleLogs(client)

	case "stop":
		fmt.Print("\nStopping Baleen Engine")
		clientHandleStop(client)
		fmt.Println("\nBaleen Engine has been stopped.")
		rl.Close()
		os.Exit(0)

	case "exit":
		fmt.Println("\nDisconnected from Baleen Engine. The background service keeps running.")
		rl.Close()
		os.Exit(0)
	default:
		fmt.Println("Unknown command. Try 'push <NODE> <IMAGE>', 'peers', 'history', 'gc <all|old|hash>', 'prune', 'stop', 'exit'")
	}
}

// Individual command implementations

func clientHandlePeers(client *daemonClient) {
	resp, err := client.get("/api/peers")
	if err != nil {
		fmt.Printf("Failed to fetch peers: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var peers []struct {
		Hostname string `json:"hostname"`
		IP       string `json:"ip"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&peers); err != nil {
		fmt.Printf("Failed to parse peers: %v\n", err)
		return
	}
	if len(peers) == 0 {
		fmt.Println("No other peers found on the network yet.")
		return
	}
	fmt.Println("\nActive Baleen Nodes (via daemon):")
	fmt.Println("--------------------------------------------------")
	fmt.Printf("%-20s | %-20s\n", "NODE NAME", "ADDRESS (IP:PORT)")
	fmt.Println("--------------------------------------------------")
	for _, p := range peers {
		fmt.Printf("%-20s | %-20s\n", p.Hostname, p.IP)
	}
	fmt.Println("--------------------------------------------------")
}

func clientHandlePush(client *daemonClient, peer, image string) {
	fmt.Printf("\nDispatching push of '%s' to '%s' via daemon...\n", image, peer)
	resp, err := client.post("/api/push", map[string]string{
		"image":        image,
		"peer":         peer,
		"buildContext": ".",
	})
	if err != nil {
		fmt.Printf("Failed to dispatch push: %v\n", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		fmt.Printf("Daemon rejected push (status %d).\n", resp.StatusCode)
		return
	}

	fmt.Printf("Push accepted by daemon. Waiting for completion")
	clientWaitForPushCompletion(client, image)
}

func clientWaitForPushCompletion(client *daemonClient, image string) {
	timeout := time.Now().Add(10 * time.Minute)
	var lastPrintedLine string

	for time.Now().Before(timeout) {
		time.Sleep(2 * time.Second)

		// Fetch and print live logs from the daemon
		logResp, err := client.get("/api/logs")
		if err == nil {
			var logs []string
			if err := json.NewDecoder(logResp.Body).Decode(&logs); err == nil {
				startIndex := 0
				if lastPrintedLine != "" {
					// Find the last printed line in the current logs
					for i := len(logs) - 1; i >= 0; i-- {
						if logs[i] == lastPrintedLine {
							startIndex = i + 1
							break
						}
					}
				} else if len(logs) > 0 {
					// If this is the first time printing logs,
					// start from the last line to avoid flooding the console.
					lastPrintedLine = logs[len(logs)-1]
					startIndex = len(logs)
				}

				for i := startIndex; i < len(logs); i++ {
					fmt.Println(logs[i])
					lastPrintedLine = logs[i]
				}
			}
			logResp.Body.Close()
		}

		// Check ledger for completion status
		resp, err := client.get("/api/ledger")
		if err != nil {
			continue
		}

		var commits []struct {
			Image  string `json:"image"`
			Status string `json:"status"`
			Hash   string `json:"hash"`
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err := json.Unmarshal(body, &commits); err != nil {
			continue
		}

		for _, c := range commits {
			if c.Image == image {
				if c.Status != "Pending" {
					fmt.Printf("\nPush completed — status: %s (hash: %.8s)\n", c.Status, c.Hash)
					return
				}
				break
			}
		}
	}
	fmt.Println("\nTimed out waiting for push to complete. Check 'history' for status.")
}

func clientHandleHistory(client *daemonClient) {
	resp, err := client.get("/api/ledger")
	if err != nil {
		fmt.Printf("Failed to fetch ledger: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var commits []struct {
		Hash      string `json:"hash"`
		Image     string `json:"image"`
		Author    string `json:"author"`
		Timestamp string `json:"timestamp"`
		Direction string `json:"direction"`
		Status    string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
		fmt.Printf("Failed to parse ledger: %v\n", err)
		return
	}
	if len(commits) == 0 {
		fmt.Println("\nThe ledger is empty. No transfers recorded yet.")
		return
	}
	fmt.Println("\nBaleen Transfer Ledger (via daemon):")
	fmt.Println("--------------------------------------------------------------------------------------------------")
	fmt.Printf("%-20s | %-12s | %-10s | %-30s | %-10s\n", "DATE", "DIRECTION", "STATUS", "IMAGE", "HASH")
	fmt.Println("--------------------------------------------------------------------------------------------------")
	for _, c := range commits {
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

func clientHandleGC(client *daemonClient, parts []string) {
	if len(parts) < 2 {
		fmt.Println("  Usage: gc <all|old [days]|hash <hash>> [-rm]")
		return
	}
	removeCache := parts[len(parts)-1] == "-rm"
	if removeCache {
		parts = parts[:len(parts)-1]
	}

	payload := map[string]any{"removeCache": removeCache}
	switch parts[1] {
	case "all":
		payload["mode"] = "all"
	case "old":
		days := 7
		if len(parts) >= 3 {
			fmt.Sscanf(parts[2], "%d", &days)
		}
		payload["mode"] = "old"
		payload["days"] = days
	default:
		payload["mode"] = "hash"
		payload["hash"] = parts[1]
	}

	resp, err := client.post("/api/gc", payload)
	if err != nil {
		fmt.Printf("GC request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if msg, ok := result["message"]; ok {
		fmt.Println(msg)
	} else {
		fmt.Printf("GC status: %s\n", resp.Status)
	}
}

// runs docker image prune.
func clientRunPrune() {
	fmt.Println("Sweeping up old dangling Docker images...")
	output, err := exec.Command("docker", "image", "prune", "-f").CombinedOutput()
	if err != nil {
		fmt.Printf("Failed to run cleanup: %v\n", err)
		return
	}
	outputStr := strings.TrimSpace(string(output))
	fmt.Printf("\n%s\n", outputStr)
	if strings.Contains(outputStr, "Total reclaimed space: 0B") {
		fmt.Println("Note: No space was freed. Images may be in use by containers.")
	} else {
		fmt.Println("Cleanup complete.")
	}
}

func clientHandleCleanLogs(client *daemonClient, parts []string) {
	removeCache := len(parts) > 1 && parts[len(parts)-1] == "-rm"
	payload := map[string]bool{"removeCache": removeCache}

	resp, err := client.post("/api/logs/clean", payload)
	if err != nil {
		fmt.Printf("Failed to clean daemon logs: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if msg, ok := result["message"]; ok {
		fmt.Println(msg)
	} else {
		fmt.Printf("Clean logs status: %s\n", resp.Status)
	}
}

func clientHandleLogs(client *daemonClient) {
	resp, err := client.get("/api/logs")
	if err != nil {
		fmt.Printf("Failed to fetch daemon logs: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var logs []string
	if err := json.NewDecoder(resp.Body).Decode(&logs); err != nil {
		fmt.Printf("Failed to parse logs: %v\n", err)
		return
	}

	if len(logs) == 0 {
		fmt.Println("Daemon log is empty.")
		return
	}

	for _, line := range logs {
		fmt.Println(line)
	}
}

func clientHandleStop(client *daemonClient) {
	resp, err := client.post("/api/stop", nil)
	if err != nil {
		fmt.Printf("\nFailed to request stop: %v\n", err)
		return
	}
	resp.Body.Close()

	// Poll /api/health until the daemon is actually gone (up to 10 s).
	timeout := time.Now().Add(10 * time.Second)
	for time.Now().Before(timeout) {
		time.Sleep(300 * time.Millisecond)
		fmt.Print(".")
		if !client.isHealthy() {
			return
		}
	}
	// If we reach here, the daemon is still alive after 10 seconds.
}

func printClientWelcome(state service.ServiceState) {
	fmt.Printf("\nConnected to Baleen Engine (Node: %s, Port: %d, running in background)\n", state.NodeName, state.Port)
	fmt.Println("The engine keeps running even after you exit this terminal.")
	fmt.Println("Commands:")
	fmt.Println("  push <NODE_NAME_OR_IP:PORT> <IMAGE>  - Send a Docker image to target")
	fmt.Println("  peers                                - Show active nodes on network")
	fmt.Println("  history                              - View the transfer ledger")
	fmt.Println("  gc <all|old|hash> [-rm]              - Run garbage collection")
	fmt.Println("  prune                                - Clean up old docker images")
	fmt.Println("  logs                                 - View recent daemon logs")
	fmt.Println("  clean-logs [-rm]                     - Truncate daemon log (use -rm to delete)")
	fmt.Println("  stop                                 - Stop the background service and exit")
	fmt.Println("  exit                                 - Disconnect CLI (engine keeps running)")
}
