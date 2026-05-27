package transfer

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// reads a full Docker tarball and streams a lightweight version
// skipping the heavy layer.tar files that the receiver already owns.
func PruneTarball(inPath string, outPath string, allDigests []string, missingDigests []string) error {
	inFile, err := os.Open(inPath)
	if err != nil {
		return err
	}
	defer inFile.Close()

	ownedMap := make(map[string]bool)
	for _, d := range allDigests {
		ownedMap[d] = true
	}
	for _, m := range missingDigests {
		ownedMap[m] = false
	}

	tr := tar.NewReader(inFile)
	var manifests []struct {
		Config string   `json:"Config"`
		Layers []string `json:"Layers"`
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if hdr.Name == "manifest.json" {
			json.NewDecoder(tr).Decode(&manifests)
			break
		}
	}

	// Safely find the main image manifest
	var mainLayers []string
	for _, m := range manifests {
		if len(m.Layers) > len(mainLayers) {
			mainLayers = m.Layers
		}
	}

	if len(mainLayers) != len(allDigests) {
		return fmt.Errorf("manifest layer count mismatch: cannot prune securely")
	}

	skipFiles := make(map[string]bool)
	for i, physicalPath := range mainLayers {
		if ownedMap[allDigests[i]] {
			skipFiles[physicalPath] = true
		}
	}

	inFile.Seek(0, 0)
	tr = tar.NewReader(inFile)
	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	tw := tar.NewWriter(outFile)
	defer tw.Close()

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if skipFiles[hdr.Name] {
			fmt.Printf("Pruning duplicate layer payload: %s\n", hdr.Name)
			continue
		}

		tw.WriteHeader(hdr)
		io.Copy(tw, tr)
	}
	return nil
}
