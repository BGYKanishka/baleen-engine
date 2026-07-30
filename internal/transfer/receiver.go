package transfer

import (
	"context"
	"encoding/json"
	"errors"
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
func StartReceiver(ctx context.Context, wg *sync.WaitGroup, listener net.Listener, incomingDir string, approvalChan chan ApprovalRequest, downloadedChan chan DownloadResult, engineLedger *ledger.Ledger, activeTransfers *atomic.Int32, accepting *atomic.Bool) {
	defer wg.Done()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
				slog.Error("failed to accept connection", "error", err)
				continue
			}
		}

		// If broadcast presence is disabled, refuse the transfer immediately.
		if accepting != nil && !accepting.Load() {
			go rejectConnection(conn)
			continue
		}

		go handleIncomingTransfer(conn, incomingDir, approvalChan, downloadedChan, engineLedger, activeTransfers)
	}
}

// sends a clean rejection response and closes the connection.
func rejectConnection(conn net.Conn) {
	defer conn.Close()
	conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	_ = json.NewEncoder(conn).Encode(TransferResponse{Approved: false})
	slog.Info("rejected incoming transfer: broadcast presence is disabled")
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

	var req TransferRequest
	if err := decoder.Decode(&req); err != nil {
		return
	}

	if req.IsControl {
		switch req.Action {
		case "pause":
			if req.Initiator == "sender" {
				// Sender paused. The data stream will stall naturally.
				// Update UI status without blocking the read loop.
				if pw, ok := GlobalManager.Get(req.ImageName, req.Author); ok {
					pw.NotifyPausedBy("sender")
				}
			} else {
				if pw, ok := GlobalManager.Get(req.ImageName, req.Author); ok {
					pw.Pause(req.Initiator)
				}
			}
		case "resume":
			if req.Initiator == "sender" {
				// Sender resumed. Clear the paused state.
				if pw, ok := GlobalManager.Get(req.ImageName, req.Author); ok {
					pw.NotifyResumed()
				}
			} else {
				if pw, ok := GlobalManager.Get(req.ImageName, req.Author); ok {
					pw.Resume(req.Initiator)
				}
			}
		case "cancel":
			if pw, ok := GlobalManager.Get(req.ImageName, req.Author); ok {
				pw.Cancel()
			} else {
				// No active transfer. Check if it's a pre-streaming cancel.
				if peer, ok := GlobalManager.CancelApproval(req.ImageName); ok {
					PublishStatus(req.ImageName, peer, "pull", "cancelled")
				}
			}
		}
		return
	}

	// We'll publish status inside receiveAndApprove based on whether it auto-approves or blocks

	ok := receiveAndApprove(req, encoder, approvalChan)
	if !ok {
		recordCommitWithStatus(req, engineLedger, "Rejected")
		return
	}

	_, _, dbPath, err := config.SetupBaleenDirectory()
	if err != nil {
		slog.Error("error getting directories", "error", err)
		recordCommitWithStatus(req, engineLedger, "Failed")
		return
	}

	missingLayers, alreadyOwnedLayers := partitionLayers(req.Layers, engineLedger)

	if err := encoder.Encode(TransferResponse{Approved: true, MissingLayers: missingLayers}); err != nil {
		slog.Error("failed to send negotiation response", "error", err)
		recordCommitWithStatus(req, engineLedger, "Failed")
		return
	}

	layerCacheDir := filepath.Join(filepath.Dir(dbPath), "layers")

	targetPath, err := downloadPayload(decoder, conn, incomingDir, req.ImageName, req.Author)
	if err != nil {
		slog.Error("error occurred", "error", err)
		GlobalHub.Publish(ProgressEvent{
			Direction: "pull", Image: req.ImageName, Peer: req.Author,
			Progress: 0, Speed: "", Status: "failed",
		})
		recordCommitWithStatus(req, engineLedger, ParseErrorToStatus(err))
		return
	}

	reconstructedPath, err := reconstructTarball(targetPath, incomingDir, req, layerCacheDir, alreadyOwnedLayers)
	if err != nil {
		slog.Error("error occurred", "error", err)
		os.Remove(targetPath)
		recordCommitWithStatus(req, engineLedger, ParseErrorToStatus(err))
		return
	}
	os.Remove(targetPath)

	warnOnArchMismatch(req)

	updateCacheAndLedger(reconstructedPath, layerCacheDir, req.Layers, missingLayers, engineLedger)

	recordCommitWithStatus(req, engineLedger, "Completed")

	downloadedChan <- DownloadResult{
		Path:      reconstructedPath,
		ImageName: req.ImageName,
	}
}

// asks for user approval for the decoded TransferRequest
func receiveAndApprove(req TransferRequest, encoder *json.Encoder, approvalChan chan ApprovalRequest) bool {
	// Check if auto-approve is enabled
	settings := config.LoadTransferSettings()
	if settings.AutoApprove {
		slog.Info("auto-approving transfer based on settings", "image", req.ImageName, "author", req.Author)
		PublishStatus(req.ImageName, req.Author, "pull", "auto-approved, waiting for sender")
		return true
	}

	PublishStatus(req.ImageName, req.Author, "pull", "waiting for approval")

	// Buffered (size 1) so CancelApproval can inject a rejection.
	respChan := make(chan bool, 1)

	// Register for external cancellation.
	GlobalManager.RegisterApproval(req.ImageName, req.Author, respChan)
	defer GlobalManager.UnregisterApproval(req.ImageName)

	approval := ApprovalRequest{Req: req, Response: respChan}
	approvalChan <- approval

	// Block until approve, reject, or cancel.
	if approved := <-respChan; !approved {
		encoder.Encode(TransferResponse{Approved: false})
		slog.Info("transfer rejected", "image", req.ImageName, "author", req.Author)
		return false
	}
	return true
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
	pw.ControlConn = conn
	defer pw.Cleanup()

	settings := config.LoadTransferSettings()
	var reader io.Reader = conn
	if settings.MaxBandwidth > 0 {
		reader = NewThrottledReader(conn, settings.MaxBandwidth*1024*1024)
	}

	bytesReceived, err := io.Copy(pw, reader)
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
func recordCommitWithStatus(req TransferRequest, engineLedger *ledger.Ledger, status string) {
	commit := ledger.Commit{
		Hash:      req.Hash,
		Image:     req.ImageName,
		Author:    req.Author,
		Timestamp: time.Now().Format(time.RFC3339),
		Direction: "Imported",
		Status:    status,
	}
	if err := engineLedger.RecordCommit(commit); err != nil {
		slog.Error("failed to write transfer to ledger", "error", err)
	}
}
