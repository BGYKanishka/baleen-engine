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

// takes the incoming lightweight tarball, injects missing layers
func StitchTarball(prunedPath string, reconstructedPath string, layerCacheDir string, allDigests []string, alreadyOwnedDigests []string) error {
	prunedFile, err := os.Open(prunedPath)
	if err != nil {
		return err
	}
	defer prunedFile.Close()

	// Read manifest to map digests to physical paths
	tr := tar.NewReader(prunedFile)
	var manifests []struct {
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

	var mainLayers []string
	for _, m := range manifests {
		if len(m.Layers) > len(mainLayers) {
			mainLayers = m.Layers
		}
	}

	pathMap := make(map[string]string)
	for i, physicalPath := range mainLayers {
		pathMap[allDigests[i]] = physicalPath
	}

	//Copy ALL files exactly as they are
	prunedFile.Seek(0, 0)
	tr = tar.NewReader(prunedFile)

	finalFile, err := os.Create(reconstructedPath)
	if err != nil {
		return err
	}
	defer finalFile.Close()

	tw := tar.NewWriter(finalFile)
	defer tw.Close()

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		tw.WriteHeader(hdr)
		io.Copy(tw, tr)
	}

	// Inject the cached layers
	for _, digest := range alreadyOwnedDigests {
		safeDigest := strings.ReplaceAll(digest, ":", "-")
		cachedLayerPath := filepath.Join(layerCacheDir, safeDigest, "layer.tar")

		layerFile, err := os.Open(cachedLayerPath)
		if err != nil {
			return err
		}

		stat, _ := layerFile.Stat()
		correctInternalPath := pathMap[digest]

		// Create a new tar header for the injected file
		hdr := &tar.Header{
			Name: filepath.ToSlash(correctInternalPath),
			Mode: 0644,
			Size: stat.Size(),
		}

		tw.WriteHeader(hdr)
		fmt.Printf("Stitching cached layer back into payload: %s\n", hdr.Name)
		io.Copy(tw, layerFile)
		layerFile.Close()
	}
	return nil
}
