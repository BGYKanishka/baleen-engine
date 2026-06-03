package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/http"
	"runtime"

	"github.com/BGYKanishka/baleen-engine/internal/cli"
	"github.com/BGYKanishka/baleen-engine/internal/config"
	"github.com/BGYKanishka/baleen-engine/internal/ledger"
	"github.com/BGYKanishka/baleen-engine/internal/network"
	"github.com/BGYKanishka/baleen-engine/internal/transfer"
)

func main() {
	nodeName := flag.String("name", "auto", "Name of the Baleen Node (leave blank for random)")
	port := flag.Int("port", 0, "Port for the Baleen Node (0 for auto-assign)")
	flag.Parse()

	finalName := *nodeName
	if finalName == "auto" {
		finalName = config.GenerateNodeName()
	}

	fmt.Println("Generating ephemeral TLS certificates for secure transfers...")
	tlsConfig, err := network.GenerateEphemeralTLS()
	if err != nil {
		panic(fmt.Errorf("failed to generate TLS config: %w", err))
	}

	//Start the TLS Listener and grab the port
	address := fmt.Sprintf(":%d", *port)
	listener, err := tls.Listen("tcp", address, tlsConfig)
	if err != nil {
		panic(fmt.Errorf("failed to bind network port: %w", err))
	}

	//Extract the actual port
	actualPort := listener.Addr().(*net.TCPAddr).Port

	fmt.Printf("Starting Baleen Engine as '%s' on Port %d...\n", finalName, actualPort)

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

	// Start Network Broadcaster using the REAL port
	go network.StartBroadcaster(finalName, actualPort)

	peerRegistry := network.NewPeerRegistry()
	go network.DiscoverPeers(finalName, peerRegistry)

	// background Checker!
	go peerRegistry.StartHealthChecker()

	// Create channels for our inputs
	approvalChan := make(chan transfer.ApprovalRequest)
	downloadedChan := make(chan string)

	go transfer.StartReceiver(listener, incomingDir, approvalChan, downloadedChan, engineLedger)

	go func() {
		metadataPort := actualPort + 1
		http.HandleFunc("/architecture", func(w http.ResponseWriter, r *http.Request) {
			arch := "linux/" + runtime.GOARCH
			w.Write([]byte(arch))
		})

		err := http.ListenAndServe(fmt.Sprintf(":%d", metadataPort), nil)
		if err != nil && err != http.ErrServerClosed {
			fmt.Println("Metadata server error:", err)
		}
	}()

	// Build the context and hand off execution to the CLI
	cliContext := cli.EngineContext{
		NodeName:       finalName,
		TempDir:        tempDir,
		ActualPort:     actualPort,
		PeerRegistry:   peerRegistry,
		EngineLedger:   engineLedger,
		TLSConfig:      tlsConfig,
		ApprovalChan:   approvalChan,
		DownloadedChan: downloadedChan,
	}

	cli.Start(cliContext)
}
