package docker

import (
	"context"
	"io"
	"os"

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
