package transfer

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// maps the structure of Docker's internal manifest.json
type manifestItem struct {
	Config string   `json:"Config"`
	Layers []string `json:"Layers"`
}

// opens a Docker tarball and returns the parsed manifest.json entries
func readManifest(tarPath string) ([]manifestItem, error) {
	file, err := os.Open(tarPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	tr := tar.NewReader(file)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Name == "manifest.json" {
			var manifest []manifestItem
			if err := json.NewDecoder(tr).Decode(&manifest); err != nil {
				return nil, fmt.Errorf("failed to parse manifest: %w", err)
			}
			return manifest, nil
		}
	}

	return nil, fmt.Errorf("manifest.json not found in tarball")
}

// returns the manifest item with the most layers from a Docker tarball
func GetMainManifestItem(tarPath string) (*manifestItem, error) {
	manifests, err := readManifest(tarPath)
	if err != nil {
		return nil, err
	}

	var maxLayers int
	var mainItem *manifestItem

	for i, m := range manifests {
		if len(m.Layers) > maxLayers {
			maxLayers = len(m.Layers)
			mainItem = &manifests[i]
		}
	}

	if mainItem == nil {
		return nil, fmt.Errorf("no layers found in manifest")
	}

	return mainItem, nil
}
