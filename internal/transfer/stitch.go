package transfer

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// takes the incoming lightweight tarball, injects missing layers
func StitchTarball(prunedPath string, reconstructedPath string, layerCacheDir string, missingDirs []string) error {
	prunedFile, err := os.Open(prunedPath)
	if err != nil {
		return err
	}
	defer prunedFile.Close()

	finalFile, err := os.Create(reconstructedPath)
	if err != nil {
		return err
	}
	defer finalFile.Close()

	tr := tar.NewReader(prunedFile)
	tw := tar.NewWriter(finalFile)
	defer tw.Close()

	// Copy everything from the incoming pruned tarball
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := io.Copy(tw, tr); err != nil {
			return err
		}
	}

	// Inject the heavy layer.tar files
	for _, dir := range missingDirs {
		safeDigest := strings.ReplaceAll(dir, ":", "-")
		cachedLayerPath := filepath.Join(layerCacheDir, safeDigest, "layer.tar")

		layerFile, err := os.Open(cachedLayerPath)
		if err != nil {
			return fmt.Errorf("CRITICAL: Local cache missing required layer %s: %w", safeDigest, err)
		}

		stat, _ := layerFile.Stat()

		// Create a new tar header for the injected file
		hdr := &tar.Header{
			Name: filepath.Join(dir, "layer.tar"),
			Mode: 0644,
			Size: stat.Size(),
		}

		if err := tw.WriteHeader(hdr); err != nil {
			layerFile.Close()
			return err
		}

		fmt.Printf("Stitching cached layer back into payload: %s\n", hdr.Name)
		if _, err := io.Copy(tw, layerFile); err != nil {
			layerFile.Close()
			return err
		}
		layerFile.Close()
	}

	return nil
}
