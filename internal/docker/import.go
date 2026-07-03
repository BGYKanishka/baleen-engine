package docker

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types/image"
)

// reads a .tar file and loads it into the Docker
func (m *Manager) LoadImage(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	ctx := context.Background()

	resp, err := m.Cli.ImageLoad(ctx, file)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// Read from resp.Body to block until Docker finishes unpacking.
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

// loads a Docker image from a .tar file and re-tags it with the specified image name.
func (m *Manager) LoadAndTag(filePath, imageName string) error {
	if err := m.LoadImage(filePath); err != nil {
		return err
	}
	// Re-tag only when the cross-compiled temporary image exists.
	tmpName := imageName + "-baleen-tmp"
	ctx := context.Background()
	if _, _, err := m.Cli.ImageInspectWithRaw(ctx, tmpName); err != nil {
		return nil
	}

	if err := m.Cli.ImageTag(ctx, tmpName, imageName); err != nil {
		return fmt.Errorf("failed to tag image %s: %w", imageName, err)
	}

	if _, err := m.Cli.ImageRemove(ctx, tmpName, image.RemoveOptions{Force: true, PruneChildren: true}); err != nil {
		fmt.Printf("Warning: failed to remove temporary tag %s: %v\n", tmpName, err)
	}

	return nil
}
