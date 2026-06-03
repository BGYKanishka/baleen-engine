package transfer

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"

	"github.com/BGYKanishka/baleen-engine/internal/ledger"
)

// connects to the remote node / asks for permission / streams the file
func PushImage(targetIP string, port int, filePath string, imageName string, hash string, author string, imageArch string, tlsConfig *tls.Config) error {
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

	conn, err := tls.Dial("tcp", address, tlsConfig)
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
