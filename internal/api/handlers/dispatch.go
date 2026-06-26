package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/cli"
	"github.com/BGYKanishka/baleen-engine/internal/docker"
	"github.com/BGYKanishka/baleen-engine/internal/ledger"
	"github.com/BGYKanishka/baleen-engine/internal/network"
	"github.com/BGYKanishka/baleen-engine/internal/transfer"
)

func Dispatch(ctx cli.EngineContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload struct {
			Image        string `json:"image"`
			Peer         string `json:"peer"`
			BuildContext string `json:"buildContext"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		buildContext := payload.BuildContext
		if buildContext == "" {
			buildContext = "."
		}

		w.WriteHeader(http.StatusAccepted)

		go runExportPipeline(ctx, payload.Image, payload.Peer, buildContext)
	}
}

// handles the export and transfer of a Docker image to a specified peer
func runExportPipeline(ctx cli.EngineContext, image, peer, buildContext string) {
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

	targetIP, port, resolveErr := network.ResolveTargetAddress(ctx.PeerRegistry, peer)
	if resolveErr != nil {
		ctx.EngineLedger.RecordCommit(ledger.Commit{
			Hash:      tempID,
			Image:     image,
			Author:    ctx.NodeName,
			Timestamp: time.Now().Format(time.RFC3339),
			Direction: "Exported",
			Status:    fmt.Sprintf("Failed: %v", resolveErr),
		})
		return
	}

	targetArch := network.DetectRemoteArch(targetIP, port)

	cfg := docker.PreflightConfig{
		ImageName:      image,
		ExpectedTarget: targetArch,
		ExportDir:      ctx.TempDir,
		BuildContext:   buildContext,
		ForceRawExport: false,
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
