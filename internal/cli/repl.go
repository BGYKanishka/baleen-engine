package cli

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/BGYKanishka/baleen-engine/internal/docker"
	"github.com/BGYKanishka/baleen-engine/internal/ledger"
	"github.com/BGYKanishka/baleen-engine/internal/logger"
	"github.com/BGYKanishka/baleen-engine/internal/network"
	"github.com/BGYKanishka/baleen-engine/internal/transfer"
	"github.com/chzyer/readline"
)

// EngineContext holds all the dependencies the CLI needs to execute commands
type EngineContext struct {
	NodeName          string
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
		panic(err)
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
	if err := ctx.DockerManager.LoadAndTag(result.Path, result.ImageName); err != nil {
		slog.Error("failed to load image into Docker", "error", err)
	} else {
		fmt.Println("Image successfully loaded and tagged! (Type 'docker images' in another terminal to verify)")
	}
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
	fmt.Println("  exit                                - Shut down engine")
}
