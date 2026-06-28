package transfer

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Creates a minimal valid Docker image tarball.
// Structure:
//	manifest.json               — lists config + layers
//	abc123config.json           — image config with rootfs.diff_ids
//	<layerDir>/layer.tar        — one layer per digest

func buildMinimalDockerTarball(t *testing.T, layerDigests []string) string {
	t.Helper()

	dir := t.TempDir()
	outPath := filepath.Join(dir, "image.tar")
	f, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("create tarball: %v", err)
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	defer tw.Close()

	// Build physical layer paths used inside the tar.
	var physicalLayerPaths []string
	for i := range layerDigests {
		// Use a short hex suffix so paths are stable and unique.
		short := fmt.Sprintf("layer%02d", i)
		physicalLayerPaths = append(physicalLayerPaths, short+"/layer.tar")
	}

	// Write each layer.tar (minimal: a single-entry tar with dummy data).
	for i, physPath := range physicalLayerPaths {
		layerContent := buildTinyLayerTar(t, fmt.Sprintf("file%d.txt", i))

		hdr := &tar.Header{
			Name: physPath,
			Mode: 0644,
			Size: int64(len(layerContent)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write layer header: %v", err)
		}
		if _, err := tw.Write(layerContent); err != nil {
			t.Fatalf("write layer data: %v", err)
		}
	}

	// Write the image config JSON.
	configName := "abc123config.json"
	configJSON := buildConfigJSON(t, layerDigests)
	writeTarEntry(t, tw, configName, configJSON)

	// Write manifest.json.
	type manifestEntry struct {
		Config string   `json:"Config"`
		Layers []string `json:"Layers"`
	}
	manifestData, err := json.Marshal([]manifestEntry{
		{Config: configName, Layers: physicalLayerPaths},
	})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	writeTarEntry(t, tw, "manifest.json", manifestData)

	return outPath
}

// Returns a minimal tar archive with one dummy file.
func buildTinyLayerTar(t *testing.T, filename string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	content := []byte("hello from " + filename)
	hdr := &tar.Header{
		Name: filename,
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("tiny layer header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("tiny layer write: %v", err)
	}
	tw.Close()
	return buf.Bytes()
}

// Returns a Docker image config with the given layer digests.
func buildConfigJSON(t *testing.T, layerDigests []string) []byte {
	t.Helper()
	cfg := struct {
		RootFS struct {
			DiffIDs []string `json:"diff_ids"`
		} `json:"rootfs"`
	}{}
	cfg.RootFS.DiffIDs = layerDigests
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	return data
}

// writeTarEntry writes a single file entry into tw.
func writeTarEntry(t *testing.T, tw *tar.Writer, name string, data []byte) {
	t.Helper()
	hdr := &tar.Header{
		Name: name,
		Mode: 0644,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("write header %s: %v", name, err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatalf("write data %s: %v", name, err)
	}
}

// sha256Digest returns
func sha256Digest(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}
