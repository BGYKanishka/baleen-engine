package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/api"
	"github.com/BGYKanishka/baleen-engine/internal/cli"
	"github.com/BGYKanishka/baleen-engine/internal/config"
	"github.com/BGYKanishka/baleen-engine/internal/docker"
	"github.com/BGYKanishka/baleen-engine/internal/ledger"
	"github.com/BGYKanishka/baleen-engine/internal/logger"
	"github.com/BGYKanishka/baleen-engine/internal/network"
	"github.com/BGYKanishka/baleen-engine/internal/service"
	"github.com/BGYKanishka/baleen-engine/internal/transfer"
)

func main() {
	//STATUS CHECK: If the user runs `baleen status`, we check if a background service is running and print its connection info.
	if len(os.Args) > 1 && os.Args[1] == "status" {
		if existing, err := service.ReadState(); err == nil && service.IsAlive(existing) {
			out := map[string]any{
				"status":    "running",
				"port":      existing.Port,
				"token":     existing.Token,
				"node_name": existing.NodeName,
			}
			data, _ := json.Marshal(out)
			fmt.Println(string(data))
		} else {
			fmt.Println(`{"status":"stopped"}`)
		}
		return
	}

	isDaemon := len(os.Args) > 1 && os.Args[1] == "daemon"
	isBackground := service.IsBackgroundProcess()

	logger.InitLogger(isDaemon && isBackground, os.Stdout)

	// LAUNCHER MODE: If the user runs `baleen daemon` from the UI, we spawn a fully detached background process and return immediately with the connection info.
	if isDaemon && !isBackground {
		runLauncher()
		return
	}

	// BACKGROUND DAEMON : If the user runs `baleen daemon` from the CLI, we start a background daemon that listens for incoming connections and serves the HTTP API.
	var daemonToken string
	var finalName string
	var targetPort int

	if isDaemon {
		// Daemon mode: parse daemon-specific flags.
		daemonFlags := flag.NewFlagSet("daemon", flag.ExitOnError)
		daemonFlags.StringVar(&daemonToken, "token", "", "Authorization token for the UI API")
		daemonFlags.StringVar(&finalName, "name", "auto", "Name of the Baleen Node")
		daemonFlags.Parse(os.Args[2:])
		if finalName == "auto" {
			finalName = config.GenerateNodeName()
		}
		targetPort = 0
	} else {
		// CLI mode: parse standard flags.
		nodeName := flag.String("name", "auto", "Name of the Baleen Node")
		flag.Parse()
		finalName = *nodeName
		if finalName == "auto" {
			finalName = config.GenerateNodeName()
		}
	}

	// CLI CLIENT : If the user runs `baleen` without `daemon`,
	// check if a background service is running. If not, we spawn one and then connect to it.
	if !isDaemon {
		tempDir, _, _, _, err := config.SetupBaleenDirectory()
		if err != nil {
			slog.Error("failed to setup directories", "error", err)
			os.Exit(1)
		}

		existing, err := service.ReadState()
		if err != nil || !service.IsAlive(existing) {
			fmt.Printf("No background service found — spawning Baleen daemon...\n")
			// Launch the background daemon!
			// The UI's status probe will pick up this auto-generated token later if needed.
			token := "cli-" + config.GenerateNodeName()
			if err := service.LaunchBackground(token, finalName); err != nil {
				slog.Error("failed to launch background service", "error", err)
				os.Exit(1)
			}

			// Wait for it to become ready
			state, err := service.WaitForReady(30 * time.Second)
			if err != nil {
				slog.Error("background service did not become ready in time", "error", err)
				os.Exit(1)
			}
			existing = state
		}

		// Connect to the running daemon
		cli.StartClientREPL(existing, tempDir)
		return
	}

	// BACKGROUND DAEMON : If we reach this point, we are running as a background daemon .
	ok, err := service.TryAcquireLock()
	if err != nil {
		slog.Error("failed to acquire service lock", "error", err)
		os.Exit(1)
	}
	if !ok {
		// Lost the race to another background process
		slog.Info("another daemon instance is starting; waiting for it to become ready")
		state, err := service.WaitForReady(30 * time.Second)
		if err != nil {
			slog.Error("timed out waiting for peer daemon", "error", err)
			os.Exit(1)
		}
		emitReady(state.Port, state.Token, state.NodeName)
		return
	}

	// Engine startup logic
	tempDir, incomingDir, dbPath, certsDir, err := config.SetupBaleenDirectory()
	if err != nil {
		slog.Error("failed to setup directories", "error", err)
		os.Exit(1)
	}
	tlsConfig, err := network.LoadOrGenerateTLS(certsDir)
	if err != nil {
		slog.Error("failed to generate TLS config", "error", err)
		os.Exit(1)
	}
	nodeFingerprint := network.GetCertificateFingerprint(tlsConfig)

	//Start the TLS Listener and grab the port
	address := fmt.Sprintf(":%d", targetPort)
	listener, err := tls.Listen("tcp", address, tlsConfig)
	if err != nil {
		slog.Error("failed to bind network port", "error", err)
		os.Exit(1)
	}

	//Extract the actual port
	p2pPort := listener.Addr().(*net.TCPAddr).Port

	// Initialize the persistent Ledger DB
	engineLedger, err := ledger.NewLedger(dbPath)
	if err != nil {
		slog.Error("failed to open ledger database", "error", err)
		os.Exit(1)
	}
	defer engineLedger.Close()
	// Initialize Docker Manager
	dockerManager, err := docker.NewManager()
	if err != nil {
		slog.Error("failed to initialize docker client", "error", err)
		os.Exit(1)
	}

	// Set up a channel to listen for stop requests from the API.
	stopCh := make(chan struct{}, 1)

	// Set up signal handling for graceful shutdown on SIGINT or SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Bridge /api/stop into context cancellation.
	go func() {
		select {
		case <-stopCh:
			slog.Info("stop requested via API")
			stop()
		case <-ctx.Done():
		}
	}()

	var wg sync.WaitGroup
	wg.Add(4)

	// Start P2P network (mDNS broadcaster, discovery, health checker).
	go network.StartBroadcaster(ctx, &wg, finalName, p2pPort, nodeFingerprint)
	peerRegistry := network.NewPeerRegistry()
	network.LoadStaticPeers(peerRegistry)
	go network.DiscoverPeers(ctx, &wg, finalName, peerRegistry)
	go peerRegistry.StartHealthChecker(ctx, &wg)

	approvalChan := make(chan transfer.ApprovalRequest)
	downloadedChan := make(chan transfer.DownloadResult)
	activeTransfers := &atomic.Int32{}
	go transfer.StartReceiver(ctx, &wg, listener, incomingDir, approvalChan, downloadedChan, engineLedger, activeTransfers)

	// Auto-load received images into Docker.
	go func() {
		for result := range downloadedChan {
			slog.Info("Unpacking and loading image into Docker Daemon...")
			if err := dockerManager.LoadAndTag(result.Path, result.ImageName); err != nil {
				slog.Warn("Failed to load image into Docker", "error", err)
			} else {
				slog.Info("Image successfully loaded into Docker!")
				os.Remove(result.Path)
			}
		}
	}()
	cliContext := cli.EngineContext{
		NodeName:        finalName,
		TempDir:         tempDir,
		ActualPort:      p2pPort,
		PeerRegistry:    peerRegistry,
		EngineLedger:    engineLedger,
		TLSConfig:       tlsConfig,
		ApprovalChan:    approvalChan,
		PendingApproval: &cli.PendingApprovalStore{},
		DownloadedChan:  downloadedChan,
		DockerManager:   dockerManager,
		ActiveTransfers: activeTransfers,
	}

	// StartDaemonServer binds its OWN random HTTP listener.
	apiPortCh := make(chan int, 1)
	go api.StartDaemonServer(cliContext, daemonToken, stopCh, apiPortCh)

	// Block until the HTTP API is ready.
	apiPort := <-apiPortCh

	if err := service.WriteState(service.ServiceState{
		Port:      apiPort,
		Token:     daemonToken,
		PID:       os.Getpid(),
		NodeName:  finalName,
		StartedAt: time.Now(),
	}); err != nil {
		slog.Warn("failed to write service state", "error", err)
	}
	slog.Info("background service ready", "apiPort", apiPort, "p2pPort", p2pPort)

	<-ctx.Done()
	slog.Info("Shutting down Baleen Engine...")

	service.ClearState()
	service.ReleaseLock()

	listener.Close()
	wg.Wait()
	slog.Info("Shutdown complete.")
}

// Launcher logic for the UI's `baleen daemon` command. This spawns a fully detached background process and returns immediately with the connection info.
func runLauncher() {
	daemonFlags := flag.NewFlagSet("daemon", flag.ExitOnError)
	var daemonToken string
	daemonFlags.StringVar(&daemonToken, "token", "", "Authorization token for the UI API")
	var daemonName string
	daemonFlags.StringVar(&daemonName, "name", "auto", "Name of the Baleen Node")
	daemonFlags.Parse(os.Args[2:])

	// Check if a background service is already running. If so, return its connection info.
	if existing, err := service.ReadState(); err == nil && service.IsAlive(existing) {
		out := map[string]any{
			"status":    "already_running",
			"port":      existing.Port,
			"token":     existing.Token,
			"node_name": existing.NodeName,
		}
		data, _ := json.Marshal(out)
		fmt.Println(string(data))
		os.Stdout.Sync()
		return
	}

	// Spawn a fully detached background copy of ourselves.
	if err := service.LaunchBackground(daemonToken, daemonName); err != nil {
		slog.Error("failed to launch background service", "error", err)
		os.Exit(1)
	}

	// Poll until the background daemon writes service.json.
	state, err := service.WaitForReady(30 * time.Second)
	if err != nil {
		slog.Error("background service did not become ready in time", "error", err)
		os.Exit(1)
	}

	emitReady(state.Port, state.Token, state.NodeName)
}

// emitReady prints the JSON line the UI's useDaemon hook parses.
func emitReady(port int, token string, nodeName string) {
	out := map[string]any{
		"status":    "ready",
		"port":      port,
		"token":     token,
		"node_name": nodeName,
	}
	data, _ := json.Marshal(out)
	fmt.Println(string(data))
	os.Stdout.Sync()
}
