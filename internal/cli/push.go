package cli

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/BGYKanishka/baleen-engine/internal/docker"
	"github.com/chzyer/readline"
)

func handlePush(parts []string, rl *readline.Instance, inputChan chan string, syncChan chan struct{}, ctx EngineContext) {
	if len(parts) < 3 {
		fmt.Println(" Usage: push <NODE_NAME_OR_IP:PORT> <IMAGE_NAME> [OPTIONAL: PATH_TO_DOCKERFILE]")
		return
	}

	targetIP, targetPort, fingerprint, targetArch, err := ResolveAndDetect(ctx, parts[1])
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

	if targetArch == "linux/amd64" {
		fmt.Printf("Target architecture detected (or fallback): %s\n", targetArch)
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

	RecordAndPush(ctx, exportedFilePath, targetImage, finalArch, targetIP, targetPort, fingerprint, "")
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
