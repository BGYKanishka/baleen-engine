package transfer

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/config"
	"github.com/BGYKanishka/baleen-engine/internal/ledger"
)

// runs a background TCP server to listen for incoming files
func StartReceiver(listener net.Listener, incomingDir string, approvalChan chan ApprovalRequest, downloadedChan chan string, engineLedger *ledger.Ledger) {
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Failed to accept connection:", err)
			continue
		}
		go handleIncomingTransfer(conn, incomingDir, approvalChan, downloadedChan, engineLedger)
	}
}

// reads a full Docker tarball and streams
func handleIncomingTransfer(conn net.Conn, incomingDir string, approvalChan chan ApprovalRequest, downloadedChan chan string, engineLedger *ledger.Ledger) {
	defer conn.Close()

	// Use a single decoder for the entire connection lifetime to avoid
	// buffering issues when switching between two decoder instances
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	req, ok := receiveAndApprove(decoder, encoder, approvalChan)
	if !ok {
		return
	}

	_, _, dbPath, err := config.SetupBaleenDirectory()
	if err != nil {
		fmt.Println("Error getting directories:", err)
		return
	}

	missingLayers, alreadyOwnedLayers := partitionLayers(req.Layers, engineLedger)

	if err := encoder.Encode(TransferResponse{Approved: true, MissingLayers: missingLayers}); err != nil {
		fmt.Println("Failed to send negotiation response:", err)
		return
	}

	layerCacheDir := filepath.Join(filepath.Dir(dbPath), "layers")

	targetPath, err := downloadPayload(decoder, conn, incomingDir)
	if err != nil {
		fmt.Println(err)
		return
	}

	reconstructedPath, err := reconstructTarball(targetPath, incomingDir, req, layerCacheDir, alreadyOwnedLayers)
	if err != nil {
		fmt.Println(err)
		os.Remove(targetPath)
		return
	}
	os.Remove(targetPath)

	warnOnArchMismatch(req)

	updateCacheAndLedger(reconstructedPath, layerCacheDir, req.Layers, missingLayers, engineLedger)

	recordCommit(req, engineLedger)

	downloadedChan <- reconstructedPath

	go scheduleDockerRetag(req.ImageName)
}

// decodes the incoming TransferRequest, asks for user approval
func receiveAndApprove(decoder *json.Decoder, encoder *json.Encoder, approvalChan chan ApprovalRequest) (TransferRequest, bool) {
	var req TransferRequest
	if err := decoder.Decode(&req); err != nil {
		return req, false
	}

	respChan := make(chan bool)
	approvalChan <- ApprovalRequest{Req: req, Response: respChan}

	if approved := <-respChan; !approved {
		encoder.Encode(TransferResponse{Approved: false})
		fmt.Println("\nTransfer rejected.")
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
func downloadPayload(decoder *json.Decoder, conn net.Conn, incomingDir string) (string, error) {
	var streamHeader StreamHeader
	if err := decoder.Decode(&streamHeader); err != nil {
		return "", fmt.Errorf("failed to read stream header: %w", err)
	}

	safeFilename := "incoming_pruned_" + streamHeader.PrunedHash[:8] + ".tar"
	targetPath := filepath.Join(incomingDir, safeFilename)

	file, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to create incoming file: %w", err)
	}

	fmt.Printf("\nDownloading optimized payload to: %s\n", targetPath)
	bytesReceived, err := io.Copy(file, conn)
	file.Close()
	if err != nil {
		os.Remove(targetPath)
		return "", fmt.Errorf("file stream failed: %w", err)
	}

	fmt.Println("Verifying payload integrity...")
	actualHash, err := ledger.GenerateHash(targetPath)
	if err != nil {
		os.Remove(targetPath)
		return "", fmt.Errorf("failed to calculate checksum: %w", err)
	}

	if actualHash != streamHeader.PrunedHash {
		os.Remove(targetPath)
		return "", fmt.Errorf("INTEGRITY FAILURE: Checksum mismatch!\nExpected: %s\nActual:   %s\nThe payload was corrupted during network transit. Deleting file...", streamHeader.PrunedHash, actualHash)
	}

	fmt.Println("Integrity verified. Payload is mathematically identical to source.")
	fmt.Printf("Successfully received %d MB!\n", bytesReceived/1024/1024)
	return targetPath, nil
}

// stitches the full tarball back together by copying in the missing layers
func reconstructTarball(targetPath string, incomingDir string, req TransferRequest, layerCacheDir string, alreadyOwnedLayers []string) (string, error) {
	reconstructedPath := filepath.Join(incomingDir, "ready_"+req.Hash[:8]+".tar")

	fmt.Println("Stitching cached layers back into payload...")
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

	fmt.Println("\n==================================================")
	fmt.Println("ARCHITECTURE MISMATCH DETECTED ON ARRIVAL ")
	fmt.Println("==================================================")
	fmt.Printf(" This image is built for '%s'.\n", req.ImageArch)
	fmt.Printf(" Your machine is running '%s'.\n", localArch)
	fmt.Println(" Docker will use QEMU/Rosetta emulation to run it.")
	fmt.Println("\n To start this container, use the following command:")
	fmt.Printf("   docker run --platform %s %s\n", req.ImageArch, req.ImageName)
	fmt.Println("\n==================================================")
}

// extracts new layers from the rebuilt tarball and records
func updateCacheAndLedger(reconstructedPath string, layerCacheDir string, allLayers []string, missingLayers []string, engineLedger *ledger.Ledger) {
	if len(missingLayers) == 0 {
		return
	}

	fmt.Println("Extracting new layers to local cache...")
	if err := ExtractAndCacheLayers(reconstructedPath, layerCacheDir, allLayers); err != nil {
		fmt.Printf("Warning: Failed to cache new layers: %v\n", err)
		return
	}

	if err := engineLedger.MarkLayersAsOwned(missingLayers); err != nil {
		fmt.Printf("Warning: Failed to update ledger cache database: %v\n", err)
		return
	}

	fmt.Printf("Successfully cached %d new layers for future delta transfers.\n", len(missingLayers))
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
		fmt.Printf("Warning: Failed to write transfer to ledger: %v\n", err)
	}
}

// waits for Docker to finish loading the image
func scheduleDockerRetag(imageName string) {
	time.Sleep(10 * time.Second)
	tmpName := imageName + "-baleen-tmp"

	cmd := exec.Command("docker", "tag", tmpName, imageName)
	if err := cmd.Run(); err == nil {
		exec.Command("docker", "rmi", tmpName).Run()
		fmt.Printf("\n\nSuccessfully updated Docker tag for '%s'!\n", imageName)
		fmt.Println("Tip: Old <none> fallback images were kept as backups. To free up space, type 'prune'.")
		fmt.Print("\nbaleen> ")
	}
}
