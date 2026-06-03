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
func handleIncomingTransfer(conn net.Conn, incomingDir string, approvalChan chan ApprovalRequest, downloadedChan chan string, engineLedger *ledger.Ledger) {
	defer conn.Close()

	var req TransferRequest
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&req); err != nil {
		return
	}
	// Ask approval
	respChan := make(chan bool)
	approvalChan <- ApprovalRequest{
		Req:      req,
		Response: respChan,
	}
	// Wait here until the types 'y' or 'n'
	approved := <-respChan
	if !approved {
		json.NewEncoder(conn).Encode(TransferResponse{Approved: false})
		fmt.Println("\nTransfer rejected.")
		return
	}

	// Figure out what layers we actually need
	_, _, dbPath, err := config.SetupBaleenDirectory()
	if err != nil {
		fmt.Println("Error getting directories:", err)
		return
	}

	var missingLayers []string
	var alreadyOwnedLayers []string

	for _, layerDigest := range req.Layers {
		// Use the instance method instead of the static function
		hasLayer := engineLedger.HasLayer(layerDigest)
		if !hasLayer {
			missingLayers = append(missingLayers, layerDigest)
		} else {
			alreadyOwnedLayers = append(alreadyOwnedLayers, layerDigest)
		}
	}

	// Send the negotiation response
	response := TransferResponse{
		Approved:      true,
		MissingLayers: missingLayers,
	}
	json.NewEncoder(conn).Encode(response)

	// Read the stream hash first
	var streamHeader StreamHeader
	if err := json.NewDecoder(conn).Decode(&streamHeader); err != nil {
		fmt.Println("Failed to read stream header:", err)
		return
	}
	// Save as a temporary pruned file
	safeFilename := "incoming_pruned_" + streamHeader.PrunedHash[:8] + ".tar"
	targetPath := filepath.Join(incomingDir, safeFilename)

	file, err := os.Create(targetPath)
	if err != nil {
		fmt.Println("Failed to create incoming file:", err)
		return
	}

	fmt.Printf("\nDownloading optimized payload to: %s\n", targetPath)

	bytesReceived, err := io.Copy(file, conn)
	file.Close()

	if err != nil {
		fmt.Println("File stream failed:", err)
		return
	}

	//Verify the integrity of the received file using the provided hash
	fmt.Println("Verifying payload integrity...")

	actualHash, err := ledger.GenerateHash(targetPath)
	if err != nil {
		fmt.Printf("Failed to calculate checksum: %v\n", err)
		os.Remove(targetPath)
		return
	}

	if actualHash != streamHeader.PrunedHash {
		fmt.Printf("INTEGRITY FAILURE: Checksum mismatch!\nExpected: %s\nActual:   %s\n", streamHeader.PrunedHash, actualHash)
		fmt.Println("The payload was corrupted during network transit. Deleting file...")
		os.Remove(targetPath)
		return
	}

	fmt.Println("Integrity verified. Payload is mathematically identical to source.")
	fmt.Printf("Successfully received %d MB!\n", bytesReceived/1024/1024)

	// Stitch the teaball back together by injecting the missing layers
	reconstructedPath := filepath.Join(incomingDir, "ready_"+req.Hash[:8]+".tar")
	layerCacheDir := filepath.Join(filepath.Dir(dbPath), "layers")

	fmt.Println("Stitching cached layers back into payload...")
	err = StitchTarball(targetPath, reconstructedPath, layerCacheDir, req.Layers, alreadyOwnedLayers)
	if err != nil {
		fmt.Println("Stitching failed:", err)
		os.Remove(targetPath)
		return
	}
	os.Remove(targetPath)

	localArch := "linux/" + runtime.GOARCH
	if req.ImageArch != "" && req.ImageArch != localArch && req.ImageArch != "unknown" {
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

	//cache extract and ledger update
	if len(missingLayers) > 0 {
		fmt.Println("Extracting new layers to local cache...")
		err = ExtractAndCacheLayers(reconstructedPath, layerCacheDir, req.Layers)
		if err != nil {
			fmt.Printf("Warning: Failed to cache new layers: %v\n", err)
		} else {
			// Use the instance method instead of the static function
			err = engineLedger.MarkLayersAsOwned(missingLayers)
			if err != nil {
				fmt.Printf("Warning: Failed to update ledger cache database: %v\n", err)
			} else {
				fmt.Printf("Successfully cached %d new layers for future delta transfers.\n", len(missingLayers))
			}
		}
	}
	// Send the fully rebuilt tarball
	downloadedChan <- reconstructedPath

	go func(targetName string) {
		time.Sleep(10 * time.Second)
		tmpName := targetName + "-baleen-tmp"

		// Ask Docker to retag the image
		cmd := exec.Command("docker", "tag", tmpName, targetName)
		if err := cmd.Run(); err == nil {
			// Delete the temporary tag
			exec.Command("docker", "rmi", tmpName).Run()

			fmt.Printf("\n\nSuccessfully updated Docker tag for '%s'!\n", targetName)
			fmt.Println("Tip: Old <none> fallback images were kept as backups. To free up space, type 'prune'.")
			fmt.Print("\nbaleen> ")
		}
	}(req.ImageName)
}
