package transfer

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

// send JSON metadata before the actual file bytes
type TransferRequest struct {
	ImageName string `json:"image"`
	Size      int64  `json:"size"`
	Hash      string `json:"hash"`
	Author    string `json:"author"`
}

// connects to the remote node / asks for permission / streams the file
func PushImage(targetIP string, port int, filePath string, imageName string, hash string, author string) error {
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
	req := TransferRequest{
		ImageName: imageName,
		Size:      fileInfo.Size(),
		Hash:      hash,
		Author:    author,
	}

	// use json Encoder to write the JSON directly into the network socket
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		return fmt.Errorf("failed to send handshake: %w", err)
	}

	// Wait for the Receiver's Approval
	fmt.Println("Waiting for receiver to approve the transfer...")

	//read exactly 2 bytes from the receiver/expecting the word "OK"
	reply := make([]byte, 2)
	_, err = io.ReadFull(conn, reply)
	if err != nil || string(reply) != "OK" {
		return fmt.Errorf("Transfer rejected by remote node")
	}

	fmt.Println("Request approved! Streaming file data...")

	// stream the actual massive file (Bypasses RAM, goes Disk -> Network)
	bytesSent, err := io.Copy(conn, file)
	if err != nil {
		return fmt.Errorf("file stream failed: %w", err)
	}

	// Calculate and print the total MB sent
	mbSent := bytesSent / 1024 / 1024
	fmt.Printf("Successfully pushed %d MB to %s!\n", mbSent, targetIP)

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
		fmt.Println("Failed to read transfer request")
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
		conn.Write([]byte("NO"))
		fmt.Println("\nTransfer rejected.")
		return
	}
	// approved it and ready for the file stream
	conn.Write([]byte("OK"))

	safeFilename := "incoming_" + req.Hash[:8] + ".tar"
	targetPath := filepath.Join(incomingDir, safeFilename)

	file, err := os.Create(targetPath)
	if err != nil {
		fmt.Println("Failed to create incoming file:", err)
		return
	}

	fmt.Printf("\nDownloading image to: %s\n", targetPath)

	bytesReceived, err := io.Copy(file, conn)

	file.Close()

	if err != nil {
		fmt.Println("File stream failed:", err)
		return
	}

	fmt.Printf("Successfully received %d MB!\n", bytesReceived/1024/1024)

	downloadedChan <- targetPath
}
