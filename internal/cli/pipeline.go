package cli

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/ledger"
	"github.com/BGYKanishka/baleen-engine/internal/network"
	"github.com/BGYKanishka/baleen-engine/internal/transfer"
)

// resolves a peer's IP/port/fingerprint and detects its target architecture.
func ResolveAndDetect(ctx EngineContext, peer string) (ip string, port int, fingerprint, arch string, err error) {
	ip, port, fingerprint, err = network.ResolveTargetAddress(ctx.PeerRegistry, peer)
	if err != nil {
		return "", 0, "", "", err
	}
	arch = network.DetectRemoteArch(ip, port)
	return ip, port, fingerprint, arch, nil
}

// writes the export to the ledger then streams it to the target node.
func RecordAndPush(ctx EngineContext, exportedPath, image, arch, targetIP string, targetPort int, fingerprint string, tempID string, peerName string) {
	fmt.Printf("Streaming image to disk at: %s\n", exportedPath)

	hash, err := ledger.GenerateHash(exportedPath)
	if err != nil {
		hash = tempID
		if hash == "" {
			hash = fmt.Sprintf("pending-%d", time.Now().UnixNano())
		}
	} else if tempID != "" {
		ctx.EngineLedger.DeleteCommit(tempID)
	}

	// Record Pending status before pushing
	ctx.EngineLedger.RecordCommit(ledger.Commit{
		Hash:      hash,
		Image:     image,
		Author:    ctx.GetNodeName(),
		Peer:      peerName,
		Timestamp: time.Now().Format(time.RFC3339),
		Direction: "Exporting",
		Status:    "Pending",
	})

	pushErr := transfer.PushImage(targetIP, targetPort, fingerprint, exportedPath, image, hash, ctx.GetNodeName(), arch, ctx.TLSConfig, peerName)

	if pushErr != nil {
		slog.Error("push failed", "error", pushErr)
	}
	commit := ledger.Commit{
		Hash:      hash,
		Image:     image,
		Author:    ctx.GetNodeName(),
		Peer:      peerName,
		Timestamp: time.Now().Format(time.RFC3339),
		Direction: "Exported",
		Status:    transfer.ParseErrorToStatus(pushErr),
	}
	if err := ctx.EngineLedger.RecordCommit(commit); err != nil {
		slog.Error("failed to write transfer to ledger", "error", err)
	}
}
