package transfer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/config"
	"github.com/BGYKanishka/baleen-engine/internal/ledger"
)

// DownloadResult holds the path to the downloaded tarball and the final image name.
type DownloadResult struct {
	Path      string
	ImageName string
}

var isHex = regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString

// runs a background TCP server to listen for incoming files
func StartReceiver(ctx context.Context, wg *sync.WaitGroup, listener net.Listener, incomingDir string, approvalChan chan ApprovalRequest, downloadedChan chan DownloadResult, engineLedger *ledger.Ledger, activeTransfers *atomic.Int32) {
	defer wg.Done()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				slog.Error("failed to accept connection", "error", err)
				continue
			}
		}
		go handleIncomingTransfer(conn, incomingDir, approvalChan, downloadedChan, engineLedger, activeTransfers)
	}
}

// reads a full Docker tarball and streams
func handleIncomingTransfer(conn net.Conn, incomingDir string, approvalChan chan ApprovalRequest, downloadedChan chan DownloadResult, engineLedger *ledger.Ledger, activeTransfers *atomic.Int32) {
	activeTransfers.Add(1)
	defer activeTransfers.Add(-1)
	defer conn.Close()

	// Use a single decoder for the entire connection lifetime to avoid
	// buffering issues when switching between two decoder instances
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	req, ok := receiveAndApprove(decoder, encoder, approvalChan)
	if !ok {
		return
	}
	//publish receiving
	PublishStatus(req.ImageName, req.Author, "pull", "waiting for approval")

	_, _, dbPath, _, err := config.SetupBaleenDirectory()
	if err != nil {
		slog.Error("error getting directories", "error", err)
		return
	}

	missingLayers, alreadyOwnedLayers := partitionLayers(req.Layers, engineLedger)

	if err := encoder.Encode(TransferResponse{Approved: true, MissingLayers: missingLayers}); err != nil {
		slog.Error("failed to send negotiation response", "error", err)
		return
	}

	layerCacheDir := filepath.Join(filepath.Dir(dbPath), "layers")

	//pass req metadata into downloadPayload
	targetPath, err := downloadPayload(decoder, conn, incomingDir, req.ImageName, req.Author)
	if err != nil {
		slog.Error("error occurred", "error", err)
		GlobalHub.Publish(ProgressEvent{
			Direction: "pull", Image: req.ImageName, Peer: req.Author,
			Progress: 0, Speed: "", Status: "failed",
		})
		return
	}

	reconstructedPath, err := reconstructTarball(targetPath, incomingDir, req, layerCacheDir, alreadyOwnedLayers)
	if err != nil {
		slog.Error("error occurred", "error", err)
		os.Remove(targetPath)
		return
	}
	os.Remove(targetPath)

	warnOnArchMismatch(req)

	updateCacheAndLedger(reconstructedPath, layerCacheDir, req.Layers, missingLayers, engineLedger)

	recordCommit(req, engineLedger)

	downloadedChan <- DownloadResult{
		Path:      reconstructedPath,
		ImageName: req.ImageName,
	}
}

// decodes the incoming TransferRequest, asks for user approval
func receiveAndApprove(decoder *json.Decoder, encoder *json.Encoder, approvalChan chan ApprovalRequest) (TransferRequest, bool) {
	var req TransferRequest
	if err := decoder.Decode(&req); err != nil {
		return req, false
	}

	respChan := make(chan bool)
	approval := ApprovalRequest{Req: req, Response: respChan}

	// Write into the channel for the store to pick up
	approvalChan <- approval

	// Block until approve/reject is called
	if approved := <-respChan; !approved {
		encoder.Encode(TransferResponse{Approved: false})
		slog.Info("transfer rejected", "image", req.ImageName, "author", req.Author)
		return req, false
	}
	return req, true
}

// checks which layers the receiver already has and splits them into two lists
func partitionLayers(layers []string, engineLedger *ledger.Ledger) (missing []string, owned []string) {
	for _, digest := range layers {
		if engineLedger.HasLayer(digest) {
			owned = append(owned, digest)
		} else {
			missing = append(missing, digest)
		}
	}
	return
}

// reads the stream header then saves the incoming bytes to a temp file
// verifying the hash before returning the path
func downloadPayload(decoder *json.Decoder, conn net.Conn, incomingDir string, image, peer string) (string, error) {
	var streamHeader StreamHeader
	if err := decoder.Decode(&streamHeader); err != nil {
		return "", fmt.Errorf("failed to read stream header: %w", err)
	}

	if !isHex(streamHeader.PrunedHash) {
		return "", fmt.Errorf("invalid hash format from peer")
	}
	safeFilename := "incoming_pruned_" + streamHeader.PrunedHash[:8] + ".tar"
	targetPath := filepath.Join(incomingDir, safeFilename)

	file, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to create incoming file: %w", err)
	}

	slog.Info("downloading optimized payload", "path", targetPath)

	//track progress
	pw := newProgressWriter(file, streamHeader.PrunedSize, image, peer, "pull")
	bytesReceived, err := io.Copy(pw, conn)
	file.Close()
	if err != nil {
		os.Remove(targetPath)
		return "", fmt.Errorf("file stream failed: %w", err)
	}

	//publish completed
	GlobalHub.Publish(ProgressEvent{
		Direction: "pull", Image: image, Peer: peer,
		Progress: 100, Speed: "0.00 MB/s", Status: "completed",
	})

	slog.Info("verifying payload integrity")
	actualHash, err := ledger.GenerateHash(targetPath)
	if err != nil {
		os.Remove(targetPath)
		return "", fmt.Errorf("failed to calculate checksum: %w", err)
	}

	if actualHash != streamHeader.PrunedHash {
		os.Remove(targetPath)
		GlobalHub.Publish(ProgressEvent{
			Direction: "pull", Image: image, Peer: peer,
			Progress: 0, Speed: "", Status: "failed",
		})
		return "", fmt.Errorf("INTEGRITY FAILURE: checksum mismatch\nExpected: %s\nActual:   %s", streamHeader.PrunedHash, actualHash)
	}

	slog.Info("successfully received payload", "sizeMB", bytesReceived/1024/1024)
	return targetPath, nil
}

// stitches the full tarball back together by copying in the missing layers
func reconstructTarball(targetPath string, incomingDir string, req TransferRequest, layerCacheDir string, alreadyOwnedLayers []string) (string, error) {
	if !isHex(req.Hash) {
		return "", fmt.Errorf("invalid hash format from peer")
	}
	reconstructedPath := filepath.Join(incomingDir, "ready_"+req.Hash[:8]+".tar")

	slog.Info("stitching cached layers back into payload")
	if err := StitchTarball(targetPath, reconstructedPath, layerCacheDir, req.Layers, alreadyOwnedLayers); err != nil {
		return "", fmt.Errorf("stitching failed: %w", err)
	}
	return reconstructedPath, nil
}

// prints a notice when the incoming image targets a different CPU architecture
func warnOnArchMismatch(req TransferRequest) {
	localArch := "linux/" + runtime.GOARCH
	if req.ImageArch == "" || req.ImageArch == localArch || req.ImageArch == "unknown" {
		return
	}

	slog.Warn("==================================================")
	slog.Warn("ARCHITECTURE MISMATCH DETECTED ON ARRIVAL")
	slog.Warn("==================================================")
	slog.Warn("image architecture mismatch", "builtFor", req.ImageArch)
	slog.Warn("local architecture", "localArch", localArch)
	slog.Warn("emulation warning")
	slog.Warn("start command hint")
	slog.Warn("docker run command", "command", "docker run --platform "+req.ImageArch+" "+req.ImageName)
	slog.Warn("==================================================")
}

// extracts new layers from the rebuilt tarball and records
func updateCacheAndLedger(reconstructedPath string, layerCacheDir string, allLayers []string, missingLayers []string, engineLedger *ledger.Ledger) {
	if len(missingLayers) == 0 {
		return
	}

	slog.Info("extracting new layers to local cache")
	if err := ExtractAndCacheLayers(reconstructedPath, layerCacheDir, allLayers); err != nil {
		slog.Error("failed to cache new layers", "error", err)
		return
	}

	if err := engineLedger.MarkLayersAsOwned(missingLayers); err != nil {
		slog.Error("failed to update ledger cache database", "error", err)
		return
	}

	slog.Info("successfully cached new layers", "count", len(missingLayers))
}

// writes a transfer entry to the ledger history
func recordCommit(req TransferRequest, engineLedger *ledger.Ledger) {
	commit := ledger.Commit{
		Hash:      req.Hash,
		Image:     req.ImageName,
		Author:    req.Author,
		Timestamp: time.Now().Format(time.RFC3339),
		Direction: "Imported",
		Status:    "Completed",
	}
	if err := engineLedger.RecordCommit(commit); err != nil {
		slog.Error("failed to write transfer to ledger", "error", err)
	}
}
