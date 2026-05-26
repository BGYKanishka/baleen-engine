package transfer

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// reads a full Docker tarball and streams a lightweight version
// skipping the heavy layer.tar files that the receiver already owns.
func PruneTarball(inPath string, outPath string, skipDirs []string) error {
	inFile, err := os.Open(inPath)
	if err != nil {
		return fmt.Errorf("failed to open source tarball: %w", err)
	}
	defer inFile.Close()

	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create pruned tarball: %w", err)
	}
	defer outFile.Close()

	tr := tar.NewReader(inFile)
	tw := tar.NewWriter(outFile)
	defer tw.Close()

	// Convert slice to map for O(1) lookups
	skipMap := make(map[string]bool)
	for _, dir := range skipDirs {
		skipMap[dir] = true
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar reading error: %w", err)
		}

		// Docker tar paths look like: "rand_id/layer.tar" or "rand_id/json"
		parts := strings.Split(filepath.ToSlash(hdr.Name), "/")
		rootDir := parts[0]

		// If this is a layer.tar file in a directory we want to skip, don't write it to the output tarball
		if len(parts) > 1 && skipMap[rootDir] && parts[1] == "layer.tar" {
			fmt.Printf("Pruning duplicate layer payload: %s\n", hdr.Name)
			continue
		}

		// Write the header and the file data
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}
		if _, err := io.Copy(tw, tr); err != nil {
			return fmt.Errorf("failed to write tar data: %w", err)
		}
	}

	return nil
}
