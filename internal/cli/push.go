package cli

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/docker"
	"github.com/BGYKanishka/baleen-engine/internal/ledger"
	"github.com/BGYKanishka/baleen-engine/internal/transfer"
	"github.com/chzyer/readline"
)

func handlePush(parts []string, rl *readline.Instance, inputChan chan string, syncChan chan struct{}, ctx EngineContext) {
	if len(parts) < 3 {
		fmt.Println(" Usage: push <NODE_NAME_OR_IP:PORT> <IMAGE_NAME> [OPTIONAL: PATH_TO_DOCKERFILE]")
		return
	}

	targetStr := resolveTargetAddress(parts[1], ctx)
	targetImage := parts[2]
	buildContext := "."
	if len(parts) >= 4 {
		buildContext = parts[3]
	}

	targetIP, targetPort := splitHostPort(targetStr)
	targetArch := detectTargetArch(targetIP, targetPort)

	fmt.Printf("Preparing to export '%s'...\n", targetImage)

	cfg := docker.PreflightConfig{
		ImageName:      targetImage,
		ExpectedTarget: targetArch,
		ExportDir:      ctx.TempDir,
		BuildContext:   buildContext,
		ForceRawExport: false,
	}

	exportedFilePath, finalArch, err := exportWithFallback(cfg, rl, inputChan, syncChan)
	if err != nil {
		fmt.Printf("Export failed: %v\n", err)
		return
	}

	recordAndPush(ctx, exportedFilePath, targetImage, finalArch, targetIP, targetPort)
}

// checks the peer registry for a named node,
// and falls back to treating the input as a raw IP:PORT.
func resolveTargetAddress(target string, ctx EngineContext) string {
	peers := ctx.PeerRegistry.GetAllPeers()
	if resolvedIP, exists := peers[target]; exists {
		fmt.Printf("\nResolved Node '%s' to %s\n", target, resolvedIP)
		return resolvedIP
	}
	return target
}

// parses "IP:PORT" or plain "IP" into separate values.
func splitHostPort(addr string) (ip string, port int) {
	port = 8080
	if strings.Contains(addr, ":") {
		parts := strings.Split(addr, ":")
		ip = parts[0]
		fmt.Sscanf(parts[1], "%d", &port)
		return
	}
	ip = addr
	return
}

// calls the remote pre-flight server to get its CPU architecture.
func detectTargetArch(ip string, port int) string {
	fmt.Printf("\nPinging %s to detect architecture...\n", ip)

	metadataURL := fmt.Sprintf("http://%s:%d/architecture", ip, port+1)
	client := http.Client{Timeout: 2 * time.Second}

	resp, err := client.Get(metadataURL)
	if err != nil {
		arch := "linux/amd64"
		fmt.Printf("Could not reach pre-flight server at %s. Falling back to %s\n", ip, arch)
		return arch
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	arch := strings.TrimSpace(string(bodyBytes))
	fmt.Printf("Target architecture detected: %s\n", arch)
	return arch
}

// runs the Docker export
func exportWithFallback(cfg docker.PreflightConfig, rl *readline.Instance, inputChan chan string, syncChan chan struct{}) (path, arch string, err error) {
	path, arch, err = docker.ExportImage(cfg)

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
		path, arch, err = docker.ExportImage(cfg)
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
		fmt.Println("Push failed:", pushErr)
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
		fmt.Printf("Warning: Failed to write transfer to ledger: %v\n", err)
	}
}
