package transfer

import (
	"archive/tar"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// takes the incoming lightweight tarball, injects missing layers
func StitchTarball(prunedPath string, reconstructedPath string, layerCacheDir string, allDigests []string, alreadyOwnedDigests []string) error {
	pathMap, err := buildDigestToPathMap(prunedPath, allDigests)
	if err != nil {
		return err
	}

	finalFile, err := os.Create(reconstructedPath)
	if err != nil {
		return err
	}
	defer finalFile.Close()

	tw := tar.NewWriter(finalFile)
	defer tw.Close()

	if err := copyAllTarEntries(prunedPath, tw); err != nil {
		return err
	}

	return injectCachedLayers(tw, layerCacheDir, alreadyOwnedDigests, pathMap)
}

// reads the manifest from a tarball and returns a mapping
func buildDigestToPathMap(tarPath string, allDigests []string) (map[string]string, error) {
	item, err := GetMainManifestItem(tarPath)
	if err != nil {
		return nil, err
	}

	mainLayers := item.Layers

	pathMap := make(map[string]string, len(allDigests))
	for i, physicalPath := range mainLayers {
		pathMap[allDigests[i]] = physicalPath
	}
	return pathMap, nil
}

// copies every entry from srcPath into tw without modification
func copyAllTarEntries(srcPath string, tw *tar.Writer) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	tr := tar.NewReader(srcFile)
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
	return nil
}

// appends locally cached layer.tar files into tw under their original internal paths
func injectCachedLayers(tw *tar.Writer, layerCacheDir string, digests []string, pathMap map[string]string) error {
	for _, digest := range digests {
		if err := injectOneLayer(tw, layerCacheDir, digest, pathMap); err != nil {
			return err
		}
	}
	return nil
}

// appends a single cached layer into tw
func injectOneLayer(tw *tar.Writer, layerCacheDir string, digest string, pathMap map[string]string) error {
	safeDigest := strings.ReplaceAll(digest, ":", "-")
	cachedLayerPath := filepath.Join(layerCacheDir, safeDigest, "layer.tar")

	layerFile, err := os.Open(cachedLayerPath)
	if err != nil {
		return err
	}
	defer layerFile.Close()

	stat, err := layerFile.Stat()
	if err != nil {
		return err
	}

	hdr := &tar.Header{
		Name: filepath.ToSlash(pathMap[digest]),
		Mode: 0644,
		Size: stat.Size(),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	slog.Info("stitching cached layer back into payload", "layer", hdr.Name)
	_, err = io.Copy(tw, layerFile)
	return err
}
