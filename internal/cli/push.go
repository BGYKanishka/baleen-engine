package cli

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/docker"
	"github.com/BGYKanishka/baleen-engine/internal/ledger"
	"github.com/BGYKanishka/baleen-engine/internal/network"
	"github.com/BGYKanishka/baleen-engine/internal/transfer"
	"github.com/chzyer/readline"
)

func handlePush(parts []string, rl *readline.Instance, inputChan chan string, syncChan chan struct{}, ctx EngineContext) {
	if len(parts) < 3 {
		fmt.Println(" Usage: push <NODE_NAME_OR_IP:PORT> <IMAGE_NAME> [OPTIONAL: PATH_TO_DOCKERFILE]")
		return
	}

	targetIP, targetPort, err := network.ResolveTargetAddress(ctx.PeerRegistry, parts[1])
	if err != nil {
		slog.Error("failed to resolve target", "error", err)
		return
	}
	if _, ok := ctx.PeerRegistry.GetAllPeers()[parts[1]]; ok {
		fmt.Printf("\nResolved Node '%s' to %s:%d\n", parts[1], targetIP, targetPort)
	}
	targetImage := parts[2]
	buildContext := "."
	if len(parts) >= 4 {
		buildContext = parts[3]
	}

	fmt.Printf("\nPinging %s to detect architecture...\n", targetIP)
	targetArch := network.DetectRemoteArch(targetIP, targetPort)
	if targetArch == "linux/amd64" {
		fmt.Printf("Could not reach pre-flight server at %s. Falling back to linux/amd64\n", targetIP)
	} else {
		fmt.Printf("Target architecture detected: %s\n", targetArch)
	}

	fmt.Printf("Preparing to export '%s'...\n", targetImage)

	cfg := docker.PreflightConfig{
		ImageName:      targetImage,
		ExpectedTarget: targetArch,
		ExportDir:      ctx.TempDir,
		BuildContext:   buildContext,
		ForceRawExport: false,
	}

	exportedFilePath, finalArch, err := exportWithFallback(ctx, cfg, rl, inputChan, syncChan)
	if err != nil {
		slog.Error("export failed", "error", err)
		return
	}

	recordAndPush(ctx, exportedFilePath, targetImage, finalArch, targetIP, targetPort)
}

// runs the Docker export
func exportWithFallback(ctx EngineContext, cfg docker.PreflightConfig, rl *readline.Instance, inputChan chan string, syncChan chan struct{}) (path, arch string, err error) {
	path, arch, err = ctx.DockerManager.ExportImage(cfg)

	if err != nil && err.Error() == "ERR_NO_DOCKERFILE" {
		fmt.Printf("\n No Dockerfile found at '%s'. Cannot autonomously cross-compile.\n", cfg.BuildContext)
		rl.SetPrompt("Enter path to your project folder, or type 'n' to send as-is: ")
		rl.Refresh()

		// Unblock feedInput to read the user's response
		syncChan <- struct{}{}
		response := strings.TrimSpace(<-inputChan)

		rl.SetPrompt("baleen> ")
		rl.Refresh()

		if strings.ToLower(response) == "n" {
			fmt.Println("\nInitiating Raw Transfer Mode (Emulation Fallback)...")
			cfg.ForceRawExport = true
		} else {
			fmt.Printf("\nRetrying cross-compilation with build context: %s\n", response)
			cfg.BuildContext = response
		}
		path, arch, err = ctx.DockerManager.ExportImage(cfg)
	}

	return
}

// writes the export to the ledger then streams it to the target node.
func recordAndPush(ctx EngineContext, exportedPath, image, arch, targetIP string, targetPort int) {
	fmt.Printf("Streaming image to disk at: %s\n", exportedPath)

	hash, _ := ledger.GenerateHash(exportedPath)
	pushErr := transfer.PushImage(targetIP, targetPort, exportedPath, image, hash, ctx.NodeName, arch, ctx.TLSConfig)

	status := "Completed"
	if pushErr != nil {
		status = "Failed"
		slog.Error("push failed", "error", pushErr)
	}
	commit := ledger.Commit{
		Hash:      hash,
		Image:     image,
		Author:    ctx.NodeName,
		Timestamp: time.Now().Format(time.RFC3339),
		Direction: "Exported",
		Status:    status,
	}
	if err := ctx.EngineLedger.RecordCommit(commit); err != nil {
		slog.Error("failed to write transfer to ledger", "error", err)
	}
}
