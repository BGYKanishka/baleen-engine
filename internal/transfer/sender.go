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

// connects to the remote node, negotiates a delta transfer, and streams the pruned payload
func PushImage(targetIP string, port int, filePath string, imageName string, hash string, author string, imageArch string, tlsConfig *tls.Config) error {
	fileSize, layers, err := inspectTarball(filePath)
	if err != nil {
		return err
	}

	conn, err := dialReceiver(targetIP, port, tlsConfig)
	if err != nil {
		return err
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	missingLayers, err := negotiate(encoder, decoder, imageName, hash, author, imageArch, fileSize, layers)
	if err != nil {
		return err
	}

	prunedPath, prunedHash, err := pruneAndHash(filePath, layers, missingLayers)
	if err != nil {
		return err
	}
	defer os.Remove(prunedPath)

	return streamPayload(encoder, conn, prunedPath, prunedHash, targetIP)
}

// opens the tarball, reads its size and layer digests
func inspectTarball(filePath string) (int64, []string, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get file info: %w", err)
	}

	layers, err := getLayersFromTarball(filePath)
	if err != nil {
		// Non-fatal: receiver will treat all layers as missing
		fmt.Printf("Warning: Failed to extract tarball layers: %v\n", err)
	}

	return info.Size(), layers, nil
}

// opens a TLS connection to the remote node
func dialReceiver(targetIP string, port int, tlsConfig *tls.Config) (*tls.Conn, error) {
	address := net.JoinHostPort(targetIP, strconv.Itoa(port))
	fmt.Printf("Connecting to remote node at %s...\n", address)

	conn, err := tls.Dial("tcp", address, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	return conn, nil
}

// sends the transfer request and returns the list of layers the receiver is missing
func negotiate(encoder *json.Encoder, decoder *json.Decoder, imageName string, hash string, author string, imageArch string, fileSize int64, layers []string) ([]string, error) {
	fmt.Println("Sending transfer request metadata...")

	req := TransferRequest{
		ImageName: imageName,
		Size:      fileSize,
		Hash:      hash,
		Author:    author,
		ImageArch: imageArch,
		Layers:    layers,
	}
	if err := encoder.Encode(req); err != nil {
		return nil, fmt.Errorf("failed to send handshake: %w", err)
	}

	fmt.Println("Waiting for receiver to approve the transfer...")

	var response TransferResponse
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to read transfer response: %w", err)
	}

	if !response.Approved {
		return nil, fmt.Errorf("transfer rejected by remote node")
	}

	fmt.Printf("Request approved! Receiver is missing %d layers. Pruning tarball...\n", len(response.MissingLayers))
	return response.MissingLayers, nil
}

// creates a pruned copy of the tarball and returns its path and hash
func pruneAndHash(filePath string, layers []string, missingLayers []string) (string, string, error) {
	prunedPath := filePath + ".pruned"

	if err := PruneTarball(filePath, prunedPath, layers, missingLayers); err != nil {
		return "", "", fmt.Errorf("failed to prune tarball: %w", err)
	}

	prunedHash, err := ledger.GenerateHash(prunedPath)
	if err != nil {
		os.Remove(prunedPath)
		return "", "", fmt.Errorf("failed to hash pruned file: %w", err)
	}

	return prunedPath, prunedHash, nil
}

// sends the stream header then copies the pruned file over the connection
func streamPayload(encoder *json.Encoder, conn *tls.Conn, prunedPath string, prunedHash string, targetIP string) error {
	info, err := os.Stat(prunedPath)
	if err != nil {
		return fmt.Errorf("failed to stat pruned file: %w", err)
	}

	header := StreamHeader{
		PrunedHash: prunedHash,
		PrunedSize: info.Size(),
	}
	if err := encoder.Encode(header); err != nil {
		return fmt.Errorf("failed to send stream header: %w", err)
	}

	fmt.Println("Streaming optimized file data...")

	prunedFile, err := os.Open(prunedPath)
	if err != nil {
		return fmt.Errorf("failed to open pruned file: %w", err)
	}
	defer prunedFile.Close()

	bytesSent, err := io.Copy(conn, prunedFile)
	if err != nil {
		return fmt.Errorf("file stream failed: %w", err)
	}

	fmt.Printf("Successfully pushed %.2f MB to %s!\n", float64(bytesSent)/1024.0/1024.0, targetIP)
	return nil
}
