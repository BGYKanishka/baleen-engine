package transfer

import (
	"archive/tar"
	"fmt"
	"io"
	"log/slog"
	"os"
)

// reads a full Docker tarball and streams
func PruneTarball(inPath string, outPath string, allDigests []string, missingDigests []string) error {
	mainLayers, err := readMainLayers(inPath)
	if err != nil {
		return err
	}

	if len(mainLayers) != len(allDigests) {
		return fmt.Errorf("manifest layer count mismatch: cannot prune securely")
	}

	skipFiles := buildSkipSet(mainLayers, allDigests, missingDigests)

	return copyTarExcluding(inPath, outPath, skipFiles)
}

// returns a set of physical tar paths that should be omitted
func buildSkipSet(mainLayers []string, allDigests []string, missingDigests []string) map[string]bool {
	missingSet := make(map[string]bool, len(missingDigests))
	for _, d := range missingDigests {
		missingSet[d] = true
	}

	skipFiles := make(map[string]bool)
	for i, physicalPath := range mainLayers {
		if !missingSet[allDigests[i]] {
			skipFiles[physicalPath] = true
		}
	}
	return skipFiles
}

// parses the manifest from a tarball and returns the physical
func readMainLayers(tarPath string) ([]string, error) {
	manifests, err := readManifest(tarPath)
	if err != nil {
		return nil, err
	}

	var mainLayers []string
	for _, m := range manifests {
		if len(m.Layers) > len(mainLayers) {
			mainLayers = m.Layers
		}
	}
	return mainLayers, nil
}

// streams inPath into a new tar at outPath, omitting entries in skipFiles
func copyTarExcluding(inPath string, outPath string, skipFiles map[string]bool) error {
	inFile, err := os.Open(inPath)
	if err != nil {
		return err
	}
	defer inFile.Close()

	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	tr := tar.NewReader(inFile)
	tw := tar.NewWriter(outFile)
	defer tw.Close()

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if skipFiles[hdr.Name] {
			slog.Info("pruning duplicate layer payload", "layer", hdr.Name)
			continue
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := io.Copy(tw, tr); err != nil {
			return err
		}
	}
	return nil
}
