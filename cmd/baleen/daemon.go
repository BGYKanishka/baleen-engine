package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
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

// runDaemon implements `baleen daemon`: it either launches a background daemon
// or runs the background daemon itself.
func runDaemon(args []string) {
	isBackground := service.IsBackgroundProcess()
	logger.InitLogger(isBackground, os.Stdout)

	if !isBackground {
		runLauncher(args)
		return
	}

	daemonFlags := flag.NewFlagSet("daemon", flag.ExitOnError)
	var daemonToken string
	daemonFlags.StringVar(&daemonToken, "token", "", "Authorization token for the UI API")
	var finalName string
	daemonFlags.StringVar(&finalName, "name", "auto", "Name of the Baleen Node")
	daemonFlags.Parse(args)
	if daemonToken == "" {
		randomBytes := make([]byte, 16)
		if _, err := rand.Read(randomBytes); err == nil {
			daemonToken = hex.EncodeToString(randomBytes)
		}
	}
	if finalName == "auto" {
		if saved := config.LoadNodeName(); saved != "" {
			finalName = saved
		} else {
			finalName = config.GenerateNodeName()
		}
	}
	targetPort := 52342

	ok, err := service.TryAcquireLock()
	if err != nil {
		slog.Error("failed to acquire service lock", "error", err)
		os.Exit(1)
	}
	if !ok {
		// Lost the race to another background process.
		slog.Info("another daemon instance is starting; waiting for it to become ready")
		state, err := service.WaitForReady(30 * time.Second)
		if err != nil {
			slog.Error("timed out waiting for peer daemon", "error", err)
			os.Exit(1)
		}
		emitReady(state.Port, state.Token, state.NodeName)
		return
	}

	// Engine startup logic.
	tempDir, incomingDir, dbPath, err := config.SetupBaleenDirectory()
	if err != nil {
		slog.Error("failed to setup directories", "error", err)
		os.Exit(1)
	}
	tlsConfig, err := network.GenerateTLS()
	if err != nil {
		slog.Error("failed to generate TLS config", "error", err)
		os.Exit(1)
	}
	nodeFingerprint := network.GetCertificateFingerprint(tlsConfig)

	// Start the TLS listener and grab the actual bound port.
	address := fmt.Sprintf(":%d", targetPort)
	listener, err := tls.Listen("tcp", address, tlsConfig)
	if err != nil {
		slog.Warn("default p2p port unavailable, falling back to random port", "error", err)
		listener, err = tls.Listen("tcp", ":0", tlsConfig)
		if err != nil {
			slog.Error("failed to bind network port", "error", err)
			os.Exit(1)
		}
	}
	p2pPort := listener.Addr().(*net.TCPAddr).Port

	// Initialize the persistent ledger DB.
	engineLedger, err := ledger.NewLedger(dbPath)
	if err != nil {
		slog.Error("failed to open ledger database", "error", err)
		os.Exit(1)
	}
	defer engineLedger.Close()

	// Clean up any pending transfers from previous runs that may have crashed
	if err := engineLedger.FailPendingTransfers(); err != nil {
		slog.Warn("failed to cleanup pending transfers", "error", err)
	}

	// Initialize Docker manager.
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

	// Load persisted network feature flags.
	netSettings := config.LoadNetworkSettings()

	var wg sync.WaitGroup
	wg.Add(2)

	// Build the peer registry and seed any static peers from the environment.
	peerRegistry := network.NewPeerRegistry()
	network.LoadStaticPeers(peerRegistry)

	// manages the mDNS discovery and broadcast goroutines.
	netController := network.NewNetworkController(
		ctx, finalName, p2pPort, nodeFingerprint, peerRegistry,
		netSettings.MDNSDiscovery, netSettings.BroadcastPresence,
	)

	// Health checker and metadata server are always active.
	go peerRegistry.StartHealthChecker(ctx, &wg)
	go network.StartMetadataServer(p2pPort+config.MetadataPortOffset, netController.BroadcastEnabledFlag())

	approvalChan := make(chan transfer.ApprovalRequest)
	downloadedChan := make(chan transfer.DownloadResult)
	activeTransfers := &atomic.Int32{}
	go transfer.StartReceiver(ctx, &wg, listener, incomingDir, approvalChan, downloadedChan, engineLedger, activeTransfers, netController.BroadcastEnabledFlag())

	// Auto-load received images into Docker.
	go func() {
		for result := range downloadedChan {
			slog.Info("Unpacking and loading image into Docker Daemon...")
			status := "completed"
			if err := dockerManager.LoadAndTag(result.Path, result.ImageName); err != nil {
				slog.Warn("Failed to load image into Docker", "error", err)
				status = "failed"
			} else {
				slog.Info("Image successfully loaded into Docker!")
				os.Remove(result.Path)
			}

			transfer.GlobalHub.Publish(transfer.ProgressEvent{
				Direction: "pull", Image: result.ImageName, Peer: result.Peer,
				Progress: 100, Speed: "0.00 MB/s", Status: status,
			})
		}
	}()

	cliContext := cli.EngineContext{
		GetNodeName:       netController.GetNodeName,
		TempDir:           tempDir,
		ActualPort:        p2pPort,
		PeerRegistry:      peerRegistry,
		EngineLedger:      engineLedger,
		TLSConfig:         tlsConfig,
		ApprovalChan:      approvalChan,
		PendingApproval:   &cli.PendingApprovalStore{},
		DownloadedChan:    downloadedChan,
		DockerManager:     dockerManager,
		ActiveTransfers:   activeTransfers,
		NetworkController: netController,
	}

	// StartDaemonServer binds its own random HTTP listener.
	apiPortCh := make(chan int, 1)
	go api.StartDaemonServer(ctx, cliContext, daemonToken, stopCh, apiPortCh)
	// Block until the HTTP API is ready.
	apiPort := <-apiPortCh
	if apiPort == 0 {
		slog.Error("daemon API server failed to bind, shutting down")
		return
	}
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

// runLauncher reports an existing daemon if available,
// otherwise it starts one and reports its connection info.
func runLauncher(args []string) {
	daemonFlags := flag.NewFlagSet("daemon", flag.ExitOnError)
	var daemonToken string
	daemonFlags.StringVar(&daemonToken, "token", "", "Authorization token for the UI API")
	var daemonName string
	daemonFlags.StringVar(&daemonName, "name", "auto", "Name of the Baleen Node")
	daemonFlags.Parse(args)

	if daemonToken == "" {
		randomBytes := make([]byte, 16)
		if _, err := rand.Read(randomBytes); err == nil {
			daemonToken = hex.EncodeToString(randomBytes)
		}
	}

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
