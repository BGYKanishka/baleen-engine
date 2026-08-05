package cli

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/BGYKanishka/baleen-engine/internal/config"
	"github.com/BGYKanishka/baleen-engine/internal/docker"
	"github.com/BGYKanishka/baleen-engine/internal/ledger"
	"github.com/BGYKanishka/baleen-engine/internal/logger"
	"github.com/BGYKanishka/baleen-engine/internal/network"
	"github.com/BGYKanishka/baleen-engine/internal/transfer"
	"github.com/chzyer/readline"
)

// EngineContext holds all the dependencies the CLI needs to execute commands
type EngineContext struct {
	GetNodeName       func() string
	TempDir           string
	ActualPort        int
	PeerRegistry      *network.PeerRegistry
	EngineLedger      *ledger.Ledger
	TLSConfig         *tls.Config
	ApprovalChan      chan transfer.ApprovalRequest
	PendingApproval   *PendingApprovalStore
	DownloadedChan    chan transfer.DownloadResult
	DockerManager     *docker.Manager
	ActiveTransfers   *atomic.Int32
	NetworkController *network.NetworkController
}

type PendingApprovalStore struct {
	mu  sync.Mutex
	val *transfer.ApprovalRequest
}

func (s *PendingApprovalStore) Store(r transfer.ApprovalRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.val = &r
}

func (s *PendingApprovalStore) Load() (transfer.ApprovalRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.val == nil {
		return transfer.ApprovalRequest{}, false
	}
	return *s.val, true
}

func (s *PendingApprovalStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.val = nil
}

func Start(ctx EngineContext) {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "baleen> ",
		HistoryFile:     filepath.Join(ctx.TempDir, "baleen_history.tmp"),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		fmt.Printf("Failed to initialize REPL: %v\n", err)
		os.Exit(1)
	}
	defer rl.Close()
	logger.InitLogger(false, rl.Stdout())

	inputChan := make(chan string)
	syncChan := make(chan struct{})

	go feedInput(rl, inputChan, syncChan)

	var pendingReq *transfer.ApprovalRequest

	printWelcome()

	for {
		select {
		case req := <-ctx.ApprovalChan:
			pendingReq = &req
			mbSize := req.Req.Size / 1024 / 1024
			fmt.Printf("\n\nINCOMING REQUEST: '%s' (%d MB) from %s\n", req.Req.ImageName, mbSize, req.Req.Author)
			rl.SetPrompt("Accept transfer? (y/n): ")
			rl.Refresh()

		case result := <-ctx.DownloadedChan:
			handleDownload(result, rl, ctx)

		case input := <-inputChan:
			input = strings.TrimSpace(input)

			if pendingReq != nil {
				resolveApproval(input, pendingReq, rl)
				pendingReq = nil
				syncChan <- struct{}{}
				continue
			}

			handleCommand(input, rl, inputChan, syncChan, ctx)
			syncChan <- struct{}{}
		}
	}
}

func feedInput(rl *readline.Instance, inputChan chan string, syncChan chan struct{}) {
	for {
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				continue
			}
			os.Exit(0)
		}
		inputChan <- line
		<-syncChan
	}
}

// loads a received image tarball into the local Docker daemon.
func handleDownload(result transfer.DownloadResult, rl *readline.Instance, ctx EngineContext) {
	fmt.Println("\nUnpacking and loading image into Docker Daemon...")
	status := "completed"
	if err := ctx.DockerManager.LoadAndTag(result.Path, result.ImageName); err != nil {
		slog.Error("failed to load image into Docker", "error", err)
		status = "failed"
	} else {
		fmt.Println("Image successfully loaded and tagged! (Type 'docker images' in another terminal to verify)")
	}

	transfer.GlobalHub.Publish(transfer.ProgressEvent{
		Direction: "pull", Image: result.ImageName, Peer: result.Peer,
		Progress: 100, Speed: "0.00 MB/s", Status: status,
	})

	rl.Refresh()
}

// sends the user's y/n response to the pending approval channel.
func resolveApproval(input string, req *transfer.ApprovalRequest, rl *readline.Instance) {
	rl.SetPrompt("")
	rl.Refresh()
	req.Response <- strings.ToLower(input) == "y"
	rl.SetPrompt("baleen> ")
	rl.Refresh()
}

func handleCleanLogs(parts []string) {
	baleenRoot, err := config.BaleenDir()
	if err != nil {
		fmt.Printf("Failed to resolve home dir: %v\n", err)
		return
	}
	logPath := filepath.Join(baleenRoot, "daemon.log")

	removeCache := len(parts) > 1 && parts[len(parts)-1] == "-rm"
	if removeCache {
		if err := os.Remove(logPath); err != nil && !os.IsNotExist(err) {
			fmt.Printf("Failed to delete log file: %v\n", err)
			return
		}
		fmt.Println("Daemon log deleted.")
		return
	}

	file, err := os.OpenFile(logPath, os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Daemon log already empty.")
			return
		}
		fmt.Printf("Failed to truncate log file: %v\n", err)
		return
	}
	file.Close()
	fmt.Println("Daemon log truncated.")
}

func handleLogs() {
	baleenRoot, err := config.BaleenDir()
	if err != nil {
		fmt.Printf("Failed to resolve home dir: %v\n", err)
		return
	}
	logPath := filepath.Join(baleenRoot, "daemon.log")

	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Daemon log is empty.")
			return
		}
		fmt.Printf("Failed to open log file: %v\n", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > 500 {
			lines = lines[1:]
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading log file: %v\n", err)
		return
	}

	if len(lines) == 0 {
		fmt.Println("Daemon log is empty.")
		return
	}

	for _, line := range lines {
		fmt.Println(line)
	}
}

// parses the input and dispatches to the correct handler.
func handleCommand(input string, rl *readline.Instance, inputChan chan string, syncChan chan struct{}, ctx EngineContext) {
	parts := strings.Split(input, " ")
	if len(parts) == 0 || parts[0] == "" {
		return
	}

	switch parts[0] {
	case "push":
		handlePush(parts, rl, inputChan, syncChan, ctx)
	case "peers":
		handlePeers(ctx.PeerRegistry)
	case "history":
		handleHistory(ctx.EngineLedger)
	case "gc":
		handleGC(parts, ctx)
	case "prune":
		handlePrune()
	case "logs":
		handleLogs()
	case "clean-logs":
		handleCleanLogs(parts)
	case "exit":
		fmt.Println("\nShutting down Baleen Engine...")
		os.Exit(0)
	default:
		fmt.Println("Unknown command. Try 'push <NODE_NAME> <IMAGE>' or 'exit'")
	}
}

func printWelcome() {
	fmt.Println("\nBaleen Engine is now online!")
	fmt.Println("Commands:")
	fmt.Println("  push <NODE_NAME_OR_IP:PORT> <IMAGE> - Send a specific Docker image to target")
	fmt.Println("  peers                               - Show active nodes on network")
	fmt.Println("  history                             - View the transfer ledger")
	fmt.Println("  gc <all|old>                        - Run garbage collection on the transfer ledger")
	fmt.Println("  prune                               - Clean up old docker images")
	fmt.Println("  logs                                - View recent daemon logs")
	fmt.Println("  clean-logs [-rm]                    - Truncate daemon log (use -rm to delete)")
	fmt.Println("  exit                                - Shut down engine")
}
