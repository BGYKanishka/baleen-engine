package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"

	"github.com/BGYKanishka/baleen-engine/internal/api"
	"github.com/BGYKanishka/baleen-engine/internal/cli"
	"github.com/BGYKanishka/baleen-engine/internal/config"
	"github.com/BGYKanishka/baleen-engine/internal/docker"
	"github.com/BGYKanishka/baleen-engine/internal/ledger"
	"github.com/BGYKanishka/baleen-engine/internal/network"
	"github.com/BGYKanishka/baleen-engine/internal/transfer"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	
	var wg sync.WaitGroup
	// Detect Daemon Mode vs Standard CLI
	isDaemon := len(os.Args) > 1 && os.Args[1] == "daemon"

	var finalName string
	var targetPort int
	var daemonToken string

	if isDaemon {
		daemonFlags := flag.NewFlagSet("daemon", flag.ExitOnError)
		daemonFlags.StringVar(&daemonToken, "token", "", "Authorization token for the UI API")
		daemonFlags.Parse(os.Args[2:])
		finalName = config.GenerateNodeName()
		targetPort = 0
	} else {
		nodeName := flag.String("name", "auto", "Name of the Baleen Node (leave blank for random)")
		port := flag.Int("port", 0, "Port for the Baleen Node (0 for auto-assign)")
		flag.Parse()

		finalName = *nodeName
		if finalName == "auto" {
			finalName = config.GenerateNodeName()
		}
		targetPort = *port
	}

	if !isDaemon {
		fmt.Println("Generating ephemeral TLS certificates for secure transfers...")
	}
	tlsConfig, err := network.GenerateEphemeralTLS()
	if err != nil {
		panic(fmt.Errorf("failed to generate TLS config: %w", err))
	}

	//Start the TLS Listener and grab the port
	address := fmt.Sprintf(":%d", targetPort)
	listener, err := tls.Listen("tcp", address, tlsConfig)
	if err != nil {
		panic(fmt.Errorf("failed to bind network port: %w", err))
	}

	//Extract the actual port
	actualPort := listener.Addr().(*net.TCPAddr).Port

	if !isDaemon {
		fmt.Printf("Starting Baleen Engine as '%s' on Port %d...\n", finalName, actualPort)
	}

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

	// Initialize Docker Manager
	dockerManager, err := docker.NewManager()
	if err != nil {
		panic(fmt.Errorf("failed to initialize docker client: %w", err))
	}

	wg.Add(4)
	// Start Network Broadcaster using the REAL port
	go network.StartBroadcaster(ctx, &wg, finalName, actualPort)

	peerRegistry := network.NewPeerRegistry()

	// Load environment variable peers fallback
	network.LoadStaticPeers(peerRegistry)

	go network.DiscoverPeers(ctx, &wg, finalName, peerRegistry)

	// background Checker!
	go peerRegistry.StartHealthChecker(ctx, &wg)

	// Create channels for our inputs
	approvalChan := make(chan transfer.ApprovalRequest)
	downloadedChan := make(chan transfer.DownloadResult)

	go transfer.StartReceiver(ctx, &wg, listener, incomingDir, approvalChan, downloadedChan, engineLedger)

	// If we're in Daemon mode, we want to automatically load downloaded images into the local Docker Daemon
	if isDaemon {
		go func() {
			for result := range downloadedChan {
				fmt.Println("Unpacking and loading image into Docker Daemon...")
				if err := dockerManager.LoadAndTag(result.Path, result.ImageName); err != nil {
					fmt.Printf("Warning: Failed to load image into Docker: %v\n", err)
				} else {
					fmt.Println("Image successfully loaded into Docker!")
					os.Remove(result.Path)
				}
			}
		}()
	}

	if !isDaemon {
		go func() {
			metadataPort := actualPort + config.MetadataPortOffset
			mux := http.NewServeMux()
			mux.HandleFunc("/architecture", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("linux/" + runtime.GOARCH))
			})

			if err := http.ListenAndServe(fmt.Sprintf(":%d", metadataPort), mux); err != nil && err != http.ErrServerClosed {
				fmt.Println("Metadata server error:", err)
			}
		}()
	}

	// Build the context and hand off execution to the CLI
	cliContext := cli.EngineContext{
		NodeName:        finalName,
		TempDir:         tempDir,
		ActualPort:      actualPort,
		PeerRegistry:    peerRegistry,
		EngineLedger:    engineLedger,
		TLSConfig:       tlsConfig,
		ApprovalChan:    approvalChan,
		PendingApproval: &cli.PendingApprovalStore{},
		DownloadedChan:  downloadedChan,
		DockerManager:   dockerManager,
	}

	if isDaemon {
		go api.StartDaemonServer(cliContext, daemonToken)
	} else {
		go cli.Start(cliContext)
	}

	<-ctx.Done()
	fmt.Println("\nShutting down Baleen Engine...")
	
	// Close listener to unblock StartReceiver
	listener.Close()
	
	wg.Wait()
	fmt.Println("Shutdown complete.")
}
