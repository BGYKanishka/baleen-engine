package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/cli"
	"github.com/BGYKanishka/baleen-engine/internal/config"
	"github.com/BGYKanishka/baleen-engine/internal/logger"
	"github.com/BGYKanishka/baleen-engine/internal/service"
)

// runClient is the default entrypoint for the `baleen` command.
// It checks for a running daemon, spawning one if needed, and then starts the interactive REPL.
func runClient(args []string) {
	logger.InitLogger(false, os.Stdout)

	clientFlags := flag.NewFlagSet("client", flag.ExitOnError)
	nodeName := clientFlags.String("name", "auto", "Name of the Baleen Node")
	clientFlags.Parse(args)

	finalName := *nodeName
	if finalName == "auto" {
		finalName = config.GenerateNodeName()
	}

	tempDir, _, _, err := config.SetupBaleenDirectory()
	if err != nil {
		slog.Error("failed to setup directories", "error", err)
		os.Exit(1)
	}

	existing, err := service.ReadState()
	if err != nil || !service.IsAlive(existing) {
		fmt.Printf("No background service found — spawning Baleen daemon...\n")

		// Generate a random auth token for the daemon we're about to spawn.
		// The UI's status probe will pick this up later via `baleen status`.
		randomBytes := make([]byte, 16)
		if _, err := rand.Read(randomBytes); err != nil {
			slog.Error("failed to generate token", "error", err)
			os.Exit(1)
		}
		token := hex.EncodeToString(randomBytes)

		if err := service.LaunchBackground(token, finalName); err != nil {
			slog.Error("failed to launch background service", "error", err)
			os.Exit(1)
		}

		// Wait for it to become ready.
		state, err := service.WaitForReady(30 * time.Second)
		if err != nil {
			slog.Error("background service did not become ready in time", "error", err)
			os.Exit(1)
		}
		existing = state
	}

	// Connect to the running daemon.
	cli.StartClientREPL(existing, tempDir)
}
