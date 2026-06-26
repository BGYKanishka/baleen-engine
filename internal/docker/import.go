package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/docker/docker/client"
)

// reads a .tar file and loads it into the Docker
func LoadImage(filePath string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	ctx := context.Background()

	resp, err := cli.ImageLoad(ctx, file)
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
func LoadAndTag(filePath, imageName string) error {
	if err := LoadImage(filePath); err != nil {
		return err
	}
	// Re-tag only when the cross-compiled temporary image exists.
	tmpName := imageName + "-baleen-tmp"
	if err := exec.Command("docker", "inspect", tmpName).Run(); err != nil {
		return nil
	}

	if err := exec.Command("docker", "tag", tmpName, imageName).Run(); err != nil {
		return fmt.Errorf("failed to tag image %s: %w", imageName, err)
	}

	if err := exec.Command("docker", "rmi", tmpName).Run(); err != nil {
		fmt.Printf("Warning: failed to remove temporary tag %s: %v\n", tmpName, err)
	}

	return nil
}
