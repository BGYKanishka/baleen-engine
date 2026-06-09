package transfer

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// unzips only the layer.tar files from the full payload and saves them in the cache
func ExtractAndCacheLayers(tarPath string, cacheDir string, expectedDigests []string) error {
	layerMap, err := buildLayerDigestMap(tarPath, expectedDigests)
	if err != nil {
		return err
	}

	return extractMatchingLayers(tarPath, cacheDir, layerMap)
}

// parses manifest.json and maps physical layer paths to their digests
func buildLayerDigestMap(tarPath string, expectedDigests []string) (map[string]string, error) {
	manifest, err := readManifest(tarPath)
	if err != nil {
		return nil, err
	}

	if len(manifest) == 0 || len(manifest[0].Layers) != len(expectedDigests) {
		return nil, fmt.Errorf("manifest layer count mismatch: cannot reliably map digests to folders")
	}

	layerMap := make(map[string]string, len(expectedDigests))
	for i, physicalPath := range manifest[0].Layers {
		layerMap[physicalPath] = expectedDigests[i]
	}
	return layerMap, nil
}

// streams the tarball and writes out only the layers present in layerMap
func extractMatchingLayers(tarPath string, cacheDir string, layerMap map[string]string) error {
	file, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer file.Close()

	tr := tar.NewReader(file)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		digest, exists := layerMap[hdr.Name]
		if !exists {
			continue
		}

		if err := writeCachedLayer(tr, cacheDir, digest); err != nil {
			return err
		}
	}
	return nil
}

// saves a single layer from the tar stream into the cache directory
func writeCachedLayer(tr *tar.Reader, cacheDir string, digest string) error {
	safeDigest := strings.ReplaceAll(digest, ":", "-")
	targetDir := filepath.Join(cacheDir, safeDigest)

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	outPath := filepath.Join(targetDir, "layer.tar")
	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	fmt.Printf("Caching layer: %s\n", digest[:15]+"...")
	_, err = io.Copy(outFile, tr)
	return err
}
