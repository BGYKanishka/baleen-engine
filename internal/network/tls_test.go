package network

import (
	"crypto/tls"
	"testing"
)

func TestGenerateTLS(t *testing.T) {
	cfg, err := GenerateTLS()
	if err != nil {
		t.Fatalf("failed to generate TLS: %v", err)
	}
	if len(cfg.Certificates) == 0 {
		t.Fatal("expected certificates to be present")
	}
	fp := GetCertificateFingerprint(cfg)
	if fp == "" {
		t.Error("expected non-empty fingerprint")
	}
}

// Each call to GenerateTLS must produce a unique cert .
func TestGenerateTLS_IsEphemeral(t *testing.T) {
	cfg1, err := GenerateTLS()
	if err != nil {
		t.Fatalf("first GenerateTLS failed: %v", err)
	}
	cfg2, err := GenerateTLS()
	if err != nil {
		t.Fatalf("second GenerateTLS failed: %v", err)
	}

	fp1 := GetCertificateFingerprint(cfg1)
	fp2 := GetCertificateFingerprint(cfg2)
	if fp1 == fp2 {
		t.Error("expected different fingerprints for separate GenerateTLS calls (ephemeral identity)")
	}
}

func TestGetCertificateFingerprint_EmptyConfig(t *testing.T) {
	cfg := &tls.Config{}
	fp := GetCertificateFingerprint(cfg)
	if fp != "" {
		t.Errorf("expected empty fingerprint for empty config, got %q", fp)
	}
}
