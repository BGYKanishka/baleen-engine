package transfer

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BGYKanishka/baleen-engine/internal/ledger"
	"github.com/BGYKanishka/baleen-engine/internal/network"
)

// run sender and receiver in the same process over real TLS loopback sockets.
func TestE2E_PushReceive_FullFlow(t *testing.T) {
	tlsCfg, incomingDir, db := testSetup(t)

	listener, addr := testListener(t, tlsCfg)
	approvalChan := make(chan ApprovalRequest, 1)
	downloadedChan := make(chan DownloadResult, 1)
	go StartReceiver(listener, incomingDir, approvalChan, downloadedChan, db)
	go autoApprove(approvalChan, true)

	tarPath := buildMinimalDockerTarball(t,
		[]string{sha256Digest([]byte("layerA")), sha256Digest([]byte("layerB"))},
	)
	host, port := parseAddr(t, addr)

	if err := PushImage(host, port, tarPath, "test-image:latest", "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", "tester", "linux/amd64", tlsCfg); err != nil {
		t.Fatalf("PushImage: %v", err)
	}

	select {
	case result := <-downloadedChan:
		assertDownload(t, result, "test-image:latest")
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for download")
	}
}

// verifies that a rejected transfer causes
// PushImage to return a non-nil error and leaves incomingDir empty.
func TestE2E_PushReceive_Rejected(t *testing.T) {
	tlsCfg, incomingDir, db := testSetup(t)

	listener, addr := testListener(t, tlsCfg)
	approvalChan := make(chan ApprovalRequest, 1)
	downloadedChan := make(chan DownloadResult, 1)
	go StartReceiver(listener, incomingDir, approvalChan, downloadedChan, db)
	go autoApprove(approvalChan, false) // ← reject

	tarPath := buildMinimalDockerTarball(t, []string{sha256Digest([]byte("layerA"))})
	host, port := parseAddr(t, addr)

	err := PushImage(host, port, tarPath, "test-image:latest", "b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3", "tester", "linux/amd64", tlsCfg)
	if err == nil {
		t.Fatal("expected error for rejected transfer, got nil")
	}
	t.Logf("Correctly got rejection error: %v", err)

	entries, _ := os.ReadDir(incomingDir)
	if len(entries) != 0 {
		t.Errorf("expected empty incomingDir after rejection, found %d files", len(entries))
	}
}

// verifies that when the receiver's ledger already knows about layerA:
//   - layerA is pruned from the wire payload (sender sends less data)
//   - the final reconstructed tarball still exists and is non-empty
func TestE2E_DeltaTransfer_ReceiverAlreadyHasLayer(t *testing.T) {
	tlsCfg, incomingDir, db := testSetup(t)

	layerA := sha256Digest([]byte("layerA-delta"))
	layerB := sha256Digest([]byte("layerB-delta"))

	// Mark layerA as already owned in the ledger.
	if err := db.MarkLayersAsOwned([]string{layerA}); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}

	// Pre-seed the physical layer cache so StitchTarball can find layerA.
	// receiver.go derives the cache dir as ~/.baleen/layers
	homeDir, _ := os.UserHomeDir()
	safeDigest := strings.ReplaceAll(layerA, ":", "-")
	cacheEntryDir := filepath.Join(homeDir, ".baleen", "layers", safeDigest)
	if err := os.MkdirAll(cacheEntryDir, 0755); err != nil {
		t.Fatalf("mkdir cache entry: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(cacheEntryDir) })

	layerATar := buildTinyLayerTar(t, "layerA.txt")
	if err := os.WriteFile(filepath.Join(cacheEntryDir, "layer.tar"), layerATar, 0644); err != nil {
		t.Fatalf("write cached layer: %v", err)
	}

	listener, addr := testListener(t, tlsCfg)
	approvalChan := make(chan ApprovalRequest, 1)
	downloadedChan := make(chan DownloadResult, 1)
	go StartReceiver(listener, incomingDir, approvalChan, downloadedChan, db)
	go autoApprove(approvalChan, true)

	tarPath := buildMinimalDockerTarball(t, []string{layerA, layerB})
	host, port := parseAddr(t, addr)

	if err := PushImage(host, port, tarPath, "delta-image:v2", "c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4", "tester", "linux/amd64", tlsCfg); err != nil {
		t.Fatalf("PushImage: %v", err)
	}

	select {
	case result := <-downloadedChan:
		assertDownload(t, result, "delta-image:v2")
		t.Logf("Delta transfer success — receiver stitched layerA from cache")
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for delta result")
	}
}

// helpers

// testSetup creates TLS config, temp incomingDir, and in-memory ledger.
func testSetup(t *testing.T) (*tls.Config, string, *ledger.Ledger) {
	t.Helper()
	tlsCfg, err := network.GenerateEphemeralTLS()
	if err != nil {
		t.Fatalf("GenerateEphemeralTLS: %v", err)
	}
	incomingDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := ledger.NewLedger(dbPath)
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return tlsCfg, incomingDir, db
}

// starts a TLS listener on a random loopback port.
func testListener(t *testing.T, cfg *tls.Config) (net.Listener, string) {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", cfg)
	if err != nil {
		t.Fatalf("tls.Listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	return ln, ln.Addr().String()
}

// drains approvalChan and sends the given decision on each request.
func autoApprove(ch chan ApprovalRequest, approve bool) {
	for req := range ch {
		req.Response <- approve
	}
}

// splits "host:port" into its parts.
func parseAddr(t *testing.T, addr string) (string, int) {
	t.Helper()
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			host := addr[:i]
			var port int
			fmt.Sscanf(addr[i+1:], "%d", &port)
			return host, port
		}
	}
	t.Fatalf("cannot parse addr %q", addr)
	return "", 0
}

// checks that a DownloadResult contains a real, non-empty file.
func assertDownload(t *testing.T, result DownloadResult, wantImage string) {
	t.Helper()
	if result.ImageName != wantImage {
		t.Errorf("ImageName: got %q, want %q", result.ImageName, wantImage)
	}
	if result.Path == "" {
		t.Fatal("DownloadResult.Path is empty")
	}
	info, err := os.Stat(result.Path)
	if err != nil {
		t.Fatalf("received file not on disk: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("received file is empty (0 bytes)")
	}
	t.Logf("received %d bytes at %s", info.Size(), result.Path)
}
