package transfer

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Maps the structure of Docker's internal manifest.json
type manifestItem struct {
	Layers []string `json:"Layers"` //
}

// unzips only the layer.tar files from the full payload and saves them in the cache
func ExtractAndCacheLayers(tarPath string, cacheDir string, expectedDigests []string) error {
	// Open tarball to find manifest.json
	file, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer file.Close()

	tr := tar.NewReader(file)
	var manifest []manifestItem

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Name == "manifest.json" {
			if err := json.NewDecoder(tr).Decode(&manifest); err != nil {
				return fmt.Errorf("failed to parse manifest: %w", err)
			}
			break
		}
	}

	if len(manifest) == 0 || len(manifest[0].Layers) != len(expectedDigests) {
		return fmt.Errorf("manifest layer count mismatch: cannot reliably map digests to folders")
	}

	// Create a fast lookup map
	layerMap := make(map[string]string)
	for i, physicalPath := range manifest[0].Layers {
		layerMap[physicalPath] = expectedDigests[i]
	}

	// Rewind the file to read layers
	file.Seek(0, 0)
	tr = tar.NewReader(file)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Check if this layer is in our manifest mapping
		if digest, exists := layerMap[hdr.Name]; exists {
			safeDigest := strings.ReplaceAll(digest, ":", "-")
			targetDir := filepath.Join(cacheDir, safeDigest)

			if err := os.MkdirAll(targetDir, 0755); err != nil {
				return err
			}

			// Create the layer.tar file in the cache
			outPath := filepath.Join(targetDir, "layer.tar")
			outFile, err := os.Create(outPath)
			if err != nil {
				return err
			}

			fmt.Printf("Caching layer: %s\n", digest[:15]+"...")
			_, err = io.Copy(outFile, tr)
			outFile.Close()

			if err != nil {
				return err
			}
		}
	}

	return nil
}
