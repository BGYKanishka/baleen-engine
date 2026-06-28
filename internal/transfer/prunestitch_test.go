package transfer

import (
	"archive/tar"
	"io"
	"os"
	"strings"
	"testing"
)

// verifies that PruneTarball drops the physical tar entry for a layer the receiver already owns,
// while keeping all other entries intact.
func TestPruneTarball_RemovesKnownLayer(t *testing.T) {
	layerA := sha256Digest([]byte("layerA"))
	layerB := sha256Digest([]byte("layerB"))
	tarPath := buildMinimalDockerTarball(t, []string{layerA, layerB})

	prunedPath := t.TempDir() + "/pruned.tar"

	// The receiver already has layerA — layerB is missing.
	// called all digests and only the missing ones.
	err := PruneTarball(tarPath, prunedPath, []string{layerA, layerB}, []string{layerB})
	if err != nil {
		t.Fatalf("PruneTarball: %v", err)
	}

	names := tarEntryNames(t, prunedPath)

	// layer00/layer.tar is layerA (owned) — must be absent.
	for _, n := range names {
		if strings.HasPrefix(n, "layer00/") {
			t.Errorf("pruned tarball still contains owned layer entry: %s", n)
		}
	}
	// layer01/layer.tar is layerB (missing) — must be present.
	found := false
	for _, n := range names {
		if strings.HasPrefix(n, "layer01/") {
			found = true
		}
	}
	if !found {
		t.Error("pruned tarball is missing the needed layer01 entry")
	}
	// manifest.json and config must still be present.
	assertEntryPresent(t, names, "manifest.json")
	assertEntryPresent(t, names, "abc123config.json")
}

// verifies nothing is pruned when all layers are missing.
func TestPruneTarball_AllMissing(t *testing.T) {
	layerA := sha256Digest([]byte("layerA"))
	layerB := sha256Digest([]byte("layerB"))
	tarPath := buildMinimalDockerTarball(t, []string{layerA, layerB})

	prunedPath := t.TempDir() + "/pruned.tar"

	err := PruneTarball(tarPath, prunedPath,
		[]string{layerA, layerB}, // all digests
		[]string{layerA, layerB}, // all missing
	)
	if err != nil {
		t.Fatalf("PruneTarball all-missing: %v", err)
	}

	names := tarEntryNames(t, prunedPath)
	assertEntryPresent(t, names, "layer00/layer.tar")
	assertEntryPresent(t, names, "layer01/layer.tar")
	assertEntryPresent(t, names, "manifest.json")
}

// verifies StitchTarball restores cached layers to the pruned tarball.
func TestStitchTarball_ReconstructsOriginalEntries(t *testing.T) {
	layerA := sha256Digest([]byte("layerA-stitch"))
	layerB := sha256Digest([]byte("layerB-stitch"))
	tarPath := buildMinimalDockerTarball(t, []string{layerA, layerB})

	dir := t.TempDir()
	prunedPath := dir + "/pruned.tar"

	// Prune layerA out (receiver owns it).
	if err := PruneTarball(tarPath, prunedPath, []string{layerA, layerB}, []string{layerB}); err != nil {
		t.Fatalf("PruneTarball: %v", err)
	}

	// Seed the layer cache with layerA.
	cacheDir := dir + "/cache"
	safeDigest := strings.ReplaceAll(layerA, ":", "-")
	layerDir := cacheDir + "/" + safeDigest
	if err := os.MkdirAll(layerDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	layerATar := buildTinyLayerTar(t, "a.txt")
	if err := os.WriteFile(layerDir+"/layer.tar", layerATar, 0644); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	reconstructedPath := dir + "/reconstructed.tar"
	err := StitchTarball(prunedPath, reconstructedPath, cacheDir, []string{layerA, layerB}, []string{layerA})
	if err != nil {
		t.Fatalf("StitchTarball: %v", err)
	}

	names := tarEntryNames(t, reconstructedPath)

	assertEntryPresent(t, names, "layer00/layer.tar")
	assertEntryPresent(t, names, "layer01/layer.tar")
	assertEntryPresent(t, names, "manifest.json")
}

// verifies PruneTarball rejects mismatched layer counts.
func TestPruneTarball_LayerCountMismatch(t *testing.T) {
	layerA := sha256Digest([]byte("only-one"))
	tarPath := buildMinimalDockerTarball(t, []string{layerA})
	prunedPath := t.TempDir() + "/pruned.tar"

	// Pass two digests for a tarball that has only one layer.
	err := PruneTarball(tarPath, prunedPath, []string{layerA, "sha256:extra"}, []string{layerA})
	if err == nil {
		t.Fatal("expected error for layer count mismatch, got nil")
	}
	t.Logf("Correctly got error: %v", err)
}

//  helpers

// returns every header Name in the tarball at path.
func tarEntryNames(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open tarball %s: %v", path, err)
	}
	defer f.Close()

	var names []string
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read tar entry: %v", err)
		}
		names = append(names, hdr.Name)
	}
	return names
}

// fails the test if name is not in names.
func assertEntryPresent(t *testing.T, names []string, name string) {
	t.Helper()
	for _, n := range names {
		if n == name {
			return
		}
	}
	t.Errorf("tarball entry %q not found; entries: %v", name, names)
}
