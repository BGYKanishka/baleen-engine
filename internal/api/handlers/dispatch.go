package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/cli"
	"github.com/BGYKanishka/baleen-engine/internal/docker"
	"github.com/BGYKanishka/baleen-engine/internal/ledger"
	"github.com/BGYKanishka/baleen-engine/internal/transfer"
)

func Dispatch(ctx cli.EngineContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload struct {
			Image string `json:"image"`
			Peer  string `json:"peer"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusAccepted)

		go runExportPipeline(ctx, payload.Image, payload.Peer)
	}
}

// handles the export and transfer of a Docker image to a specified peer
func runExportPipeline(ctx cli.EngineContext, image, peer string) {
	tempID := fmt.Sprintf("pending-%d", time.Now().UnixNano())
	ctx.EngineLedger.RecordCommit(ledger.Commit{
		Hash:      tempID,
		Image:     image,
		Author:    ctx.NodeName,
		Timestamp: time.Now().Format(time.RFC3339),
		Direction: "Exporting",
		Status:    "Pending",
	})

	// Catch any catastrophic panics and write them to the DB
	defer func() {
		if r := recover(); r != nil {
			ctx.EngineLedger.RecordCommit(ledger.Commit{
				Hash:      tempID,
				Image:     image,
				Author:    ctx.NodeName,
				Timestamp: time.Now().Format(time.RFC3339),
				Direction: "Exported",
				Status:    fmt.Sprintf("Failed (Panic): %v", r),
			})
		}
	}()

	targetIP, port := resolveTarget(ctx, peer)

	targetArch := detectArchitecture(targetIP, port)

	cfg := docker.PreflightConfig{
		ImageName:      image,
		ExpectedTarget: targetArch,
		ExportDir:      ctx.TempDir,
		BuildContext:   ".",
		ForceRawExport: true,
	}

	exportedFilePath, arch, err := docker.ExportImage(cfg)
	if err != nil {
		ctx.EngineLedger.RecordCommit(ledger.Commit{
			Hash:      tempID,
			Image:     image,
			Author:    ctx.NodeName,
			Timestamp: time.Now().Format(time.RFC3339),
			Direction: "Exported",
			Status:    fmt.Sprintf("Failed: %v", err),
		})
		return
	}

	hash, hashErr := ledger.GenerateHash(exportedFilePath)
	if hashErr != nil {
		hash = tempID
	} else {
		ctx.EngineLedger.DeleteCommit(tempID)
	}

	pushErr := transfer.PushImage(targetIP, port, exportedFilePath, image, hash, ctx.NodeName, arch, ctx.TLSConfig)

	status := "Completed"
	if pushErr != nil {
		status = fmt.Sprintf("Failed: %v", pushErr)
	}

	ctx.EngineLedger.RecordCommit(ledger.Commit{
		Hash:      hash,
		Image:     image,
		Author:    ctx.NodeName,
		Timestamp: time.Now().Format(time.RFC3339),
		Direction: "Exported",
		Status:    status,
	})
}

// resolves the peer name to an IP and port
func resolveTarget(ctx cli.EngineContext, peer string) (string, int) {
	targetIP := peer
	peers := ctx.PeerRegistry.GetAllPeers()
	if resolvedIP, exists := peers[peer]; exists {
		targetIP = resolvedIP
	}

	port := 8080
	if strings.Contains(targetIP, ":") {
		parts := strings.Split(targetIP, ":")
		targetIP = parts[0]
		fmt.Sscanf(parts[1], "%d", &port)
	}

	return targetIP, port
}

// probes the target node for its CPU architecture
func detectArchitecture(targetIP string, port int) string {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s:%d/architecture", targetIP, port+1))
	if err != nil {
		return "linux/amd64"
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil || len(bodyBytes) == 0 {
		return "linux/amd64"
	}

	return strings.TrimSpace(string(bodyBytes))
}
