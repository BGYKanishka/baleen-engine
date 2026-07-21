package network

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrGenerateTLS(t *testing.T) {
	certsDir := t.TempDir()

	// Generate new TLS certs
	cfg1, err := LoadOrGenerateTLS(certsDir)
	if err != nil {
		t.Fatalf("failed to generate TLS: %v", err)
	}
	if len(cfg1.Certificates) == 0 {
		t.Fatal("expected certificates to be loaded")
	}

	// Verify files are written
	if _, err := os.Stat(filepath.Join(certsDir, "cert.pem")); os.IsNotExist(err) {
		t.Error("cert.pem was not created")
	}
	if _, err := os.Stat(filepath.Join(certsDir, "key.pem")); os.IsNotExist(err) {
		t.Error("key.pem was not created")
	}

	// Load existing TLS certs
	cfg2, err := LoadOrGenerateTLS(certsDir)
	if err != nil {
		t.Fatalf("failed to load existing TLS: %v", err)
	}

	fp1 := GetCertificateFingerprint(cfg1)
	fp2 := GetCertificateFingerprint(cfg2)

	if fp1 == "" {
		t.Error("expected non-empty fingerprint")
	}
	if fp1 != fp2 {
		t.Errorf("expected loaded fingerprint %q to match generated %q", fp2, fp1)
	}
}

func TestGetCertificateFingerprint_EmptyConfig(t *testing.T) {
	cfg := &tls.Config{}
	fp := GetCertificateFingerprint(cfg)
	if fp != "" {
		t.Errorf("expected empty fingerprint for empty config, got %q", fp)
	}
}
