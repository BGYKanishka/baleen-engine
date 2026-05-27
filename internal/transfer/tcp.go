package transfer

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/config"
	"github.com/BGYKanishka/baleen-engine/internal/ledger"
)

// send JSON metadata before the actual file bytes
type TransferRequest struct {
	ImageName string   `json:"image"`
	Size      int64    `json:"size"`
	Hash      string   `json:"hash"`
	Author    string   `json:"author"`
	ImageArch string   `json:"image_arch"`
	Layers    []string `json:"layers"`
}

type TransferResponse struct {
	Approved      bool     `json:"approved"`
	MissingLayers []string `json:"missing_layers"`
}

type StreamHeader struct {
	PrunedHash string `json:"pruned_hash"`
	PrunedSize int64  `json:"pruned_size"`
}

// connects to the remote node / asks for permission / streams the file
func PushImage(targetIP string, port int, filePath string, imageName string, hash string, author string, imageArch string) error {
	// Open the local .tar file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get the exact file size for the handshake
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Connect to the remote Receiver via raw TCP
	address := net.JoinHostPort(targetIP, strconv.Itoa(port))
	fmt.Printf("Connecting to remote node at %s...\n", address)

	conn, err := net.Dial("tcp", address)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close()

	// Send the Metadata Handshake
	fmt.Println("Sending transfer request metadata...")

	// Extract layers directly from the generated tarball payload
	layers, err := getLayersFromTarball(filePath)
	if err != nil {
		fmt.Printf("Warning: Failed to extract tarball layers: %v\n", err)
	}

	req := TransferRequest{
		ImageName: imageName,
		Size:      fileInfo.Size(),
		Hash:      hash,
		Author:    author,
		ImageArch: imageArch,
		Layers:    layers,
	}

	// use json Encoder to write the JSON directly into the network socket
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		return fmt.Errorf("failed to send handshake: %w", err)
	}

	// Wait for the Receiver's Approval
	fmt.Println("Waiting for receiver to approve the transfer...")

	var response TransferResponse
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&response); err != nil {
		return fmt.Errorf("failed to read transfer response: %w", err)
	}

	if !response.Approved {
		return fmt.Errorf("Transfer rejected by remote node")
	}

	fmt.Printf("Request approved! Receiver is missing %d layers. Pruning tarball...\n", len(response.MissingLayers))

	// Prune the tarball locally
	prunedFilePath := filePath + ".pruned"

	// Pass ALL layers AND missing layers
	err = PruneTarball(filePath, prunedFilePath, layers, response.MissingLayers)
	if err != nil {
		return fmt.Errorf("failed to prune tarball: %w", err)
	}
	defer os.Remove(prunedFilePath)

	// Hash the new pruned file
	prunedHash, err := ledger.GenerateHash(prunedFilePath)
	if err != nil {
		return fmt.Errorf("failed to hash pruned file: %w", err)
	}

	prunedFileInfo, _ := os.Stat(prunedFilePath)

	// Send the Stream Header
	header := StreamHeader{
		PrunedHash: prunedHash,
		PrunedSize: prunedFileInfo.Size(),
	}
	if err := json.NewEncoder(conn).Encode(header); err != nil {
		return fmt.Errorf("failed to send stream header: %w", err)
	}

	fmt.Println("Streaming optimized file data...")

	// Stream the pruned file to the network
	prunedFile, err := os.Open(prunedFilePath)
	if err != nil {
		return err
	}
	defer prunedFile.Close()

	bytesSent, err := io.Copy(conn, prunedFile)
	if err != nil {
		return fmt.Errorf("file stream failed: %w", err)
	}

	// Calculate and print the total MB sent
	mbSent := float64(bytesSent) / 1024.0 / 1024.0
	fmt.Printf("Successfully pushed %.2f MB to %s!\n", mbSent, targetIP)

	return nil
}

// passes the metadata and provides a channel
type ApprovalRequest struct {
	Req      TransferRequest
	Response chan bool
}

// runs a background TCP server to listen for incoming files
func StartReceiver(port int, incomingDir string, approvalChan chan ApprovalRequest, downloadedChan chan string) {
	address := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Printf("Failed to start receiver: %v\n", err)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Failed to accept connection:", err)
			continue
		}

		go handleIncomingTransfer(conn, incomingDir, approvalChan, downloadedChan)
	}
}
func handleIncomingTransfer(conn net.Conn, incomingDir string, approvalChan chan ApprovalRequest, downloadedChan chan string) {
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
		fmt.Println("Error getting dbPath:", err)
		return
	}

	var missingLayers []string
	var alreadyOwnedLayers []string

	for _, layerDigest := range req.Layers {
		// Check your bbolt ledger
		hasLayer := ledger.HasLayer(dbPath, layerDigest)
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
			fmt.Printf("Warning: Failed to cache new layers (future delta transfers may skip these): %v\n", err)
		} else {
			err = ledger.MarkLayersAsOwned(dbPath, missingLayers)
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

func getLayersFromTarball(tarPath string) ([]string, error) {
	file, err := os.Open(tarPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	tr := tar.NewReader(file)
	var manifests []struct {
		Config string   `json:"Config"`
		Layers []string `json:"Layers"`
	}
	var configName string

	// Find manifest.json and the main image config
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if hdr.Name == "manifest.json" {
			if err := json.NewDecoder(tr).Decode(&manifests); err != nil {
				return nil, err
			}
			// Find the main image manifest
			max := 0
			for _, m := range manifests {
				if len(m.Layers) > max {
					max = len(m.Layers)
					configName = m.Config
				}
			}
			break
		}
	}

	if configName == "" {
		return nil, fmt.Errorf("could not find main config in manifest")
	}

	//Rewind and parse the Config JSON to get the actual layer digests
	file.Seek(0, 0)
	tr = tar.NewReader(file)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if hdr.Name == configName {
			var config struct {
				RootFS struct {
					DiffIDs []string `json:"diff_ids"`
				} `json:"rootfs"`
			}
			if err := json.NewDecoder(tr).Decode(&config); err != nil {
				return nil, err
			}
			return config.RootFS.DiffIDs, nil
		}
	}

	return nil, fmt.Errorf("config file not found inside tarball")
}
