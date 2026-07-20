package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/cli"
	"github.com/BGYKanishka/baleen-engine/internal/docker"
	"github.com/BGYKanishka/baleen-engine/internal/ledger"
)

func Dispatch(ctx cli.EngineContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

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
	ctx.ActiveTransfers.Add(1)
	defer ctx.ActiveTransfers.Add(-1)

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
				Status:    "Crashed",
			})
		}
	}()

	targetIP, port, fingerprint, targetArch, resolveErr := cli.ResolveAndDetect(ctx, peer)
	if resolveErr != nil {
		ctx.EngineLedger.RecordCommit(ledger.Commit{
			Hash:      tempID,
			Image:     image,
			Author:    ctx.NodeName,
			Timestamp: time.Now().Format(time.RFC3339),
			Direction: "Exported",
			Status:    "Failed",
		})
		return
	}

	cfg := docker.PreflightConfig{
		ImageName:      image,
		ExpectedTarget: targetArch,
		ExportDir:      ctx.TempDir,
		BuildContext:   buildContext,
		ForceRawExport: false,
	}

	exportedFilePath, arch, err := ctx.DockerManager.ExportImage(cfg)
	if err != nil {
		ctx.EngineLedger.RecordCommit(ledger.Commit{
			Hash:      tempID,
			Image:     image,
			Author:    ctx.NodeName,
			Timestamp: time.Now().Format(time.RFC3339),
			Direction: "Exported",
			Status:    "Failed",
		})
		return
	}

	cli.RecordAndPush(ctx, exportedFilePath, image, arch, targetIP, port, fingerprint, tempID)
}
