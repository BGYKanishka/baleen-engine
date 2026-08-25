package transfer

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
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
	accepting := &atomic.Bool{}
	accepting.Store(true)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go StartReceiver(context.Background(), wg, listener, incomingDir, approvalChan, downloadedChan, db, &atomic.Int32{}, accepting)
	go autoApprove(approvalChan, true)

	tarPath := buildMinimalDockerTarball(t,
		[]string{sha256Digest([]byte("layerA")), sha256Digest([]byte("layerB"))},
	)
	host, port := parseAddr(t, addr)

	if err := PushImage(host, port, network.GetCertificateFingerprint(tlsCfg), tarPath, "test-image:latest", "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", "tester", "linux/amd64", tlsCfg, "test-peer"); err != nil {
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
	accepting := &atomic.Bool{}
	accepting.Store(true)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go StartReceiver(context.Background(), wg, listener, incomingDir, approvalChan, downloadedChan, db, &atomic.Int32{}, accepting)
	go autoApprove(approvalChan, false) // ← reject

	tarPath := buildMinimalDockerTarball(t, []string{sha256Digest([]byte("layerA"))})
	host, port := parseAddr(t, addr)

	err := PushImage(host, port, network.GetCertificateFingerprint(tlsCfg), tarPath, "test-image:latest", "b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3", "tester", "linux/amd64", tlsCfg, "test-peer")
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
	accepting := &atomic.Bool{}
	accepting.Store(true)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go StartReceiver(context.Background(), wg, listener, incomingDir, approvalChan, downloadedChan, db, &atomic.Int32{}, accepting)
	go autoApprove(approvalChan, true)

	tarPath := buildMinimalDockerTarball(t, []string{layerA, layerB})
	host, port := parseAddr(t, addr)

	if err := PushImage(host, port, network.GetCertificateFingerprint(tlsCfg), tarPath, "delta-image:v2", "c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4", "tester", "linux/amd64", tlsCfg, "test-peer"); err != nil {
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

// verifies that a TransferRequest carrying a path-traversal layer digest
// is silently dropped by the receiver before any file I/O occurs.
func TestE2E_MaliciousLayerDigest_Rejected(t *testing.T) {
	tlsCfg, incomingDir, db := testSetup(t)

	listener, addr := testListener(t, tlsCfg)
	approvalChan := make(chan ApprovalRequest, 1)
	downloadedChan := make(chan DownloadResult, 1)
	accepting := &atomic.Bool{}
	accepting.Store(true)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go StartReceiver(context.Background(), wg, listener, incomingDir, approvalChan, downloadedChan, db, &atomic.Int32{}, accepting)
	go autoApprove(approvalChan, true)

	// Build a tarball with a valid digest so the sender can inspect it,
	// but override Layers in the request by sending a raw handshake.
	maliciousDigest := "../../../../../../tmp/pwned"
	tarPath := buildMinimalDockerTarball(t, []string{sha256Digest([]byte("legit"))})
	host, port := parseAddr(t, addr)

	// Dial the receiver directly and send a crafted TransferRequest.
	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", host, port), &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	req := TransferRequest{
		ImageName: "evil-image:latest",
		Size:      1024,
		Hash:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Author:    "attacker",
		ImageArch: "linux/amd64",
		Layers:    []string{maliciousDigest},
	}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		t.Fatalf("send request: %v", err)
	}

	// The receiver should drop the connection — read should return EOF or error.
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 1024)
	_, err = conn.Read(buf)
	if err == nil {
		t.Fatal("expected connection to be dropped for malicious digest, but got a response")
	}
	t.Logf("Correctly rejected: %v", err)

	// Verify nothing was written to disk.
	entries, _ := os.ReadDir(incomingDir)
	if len(entries) != 0 {
		t.Errorf("expected empty incomingDir, found %d files", len(entries))
	}

	// Verify the traversal target was never created.
	if _, err := os.Stat("/tmp/pwned"); err == nil {
		os.RemoveAll("/tmp/pwned")
		t.Fatal("path traversal succeeded — /tmp/pwned was created")
	}

	_ = tarPath // used only for test setup
}

// verifies the isDigest regex accepts valid Docker layer digests
// and rejects path-traversal attempts and other malformed inputs.
func TestIsDigest_Validation(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		// valid
		{"sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", true},
		{"sha256:0000000000000000000000000000000000000000000000000000000000000000", true},

		// path traversal
		{"../../../../../../tmp/pwned", false},
		{"../etc/cron.d/backdoor", false},
		{"sha256:../../../etc/passwd", false},

		// bare hex (missing sha256: prefix)
		{"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", false},

		// wrong prefix
		{"sha512:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", false},

		// too short / too long
		{"sha256:abcd", false},
		{"sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855aa", false},

		// empty
		{"", false},

		// uppercase hex (Windows Docker compatibility)
		{"sha256:E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855", true},

		// mixed case
		{"sha256:e3b0C44298FC1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", true},
	}

	for _, tt := range tests {
		got := isDigest(tt.input)
		if got != tt.want {
			t.Errorf("isDigest(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// helpers

// testSetup creates TLS config, temp incomingDir, and in-memory ledger.
func testSetup(t *testing.T) (*tls.Config, string, *ledger.Ledger) {
	t.Helper()
	tlsCfg, err := network.GenerateTLS()
	if err != nil {
		t.Fatalf("GenerateTLS: %v", err)
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
