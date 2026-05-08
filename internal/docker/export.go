package docker

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/client"
)

// saves a Docker image as a tar file and returns the saved file path.
func ExportImage(imageName string, exportDir string) (string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", err
	}
	defer cli.Close()

	ctx := context.Background()
	imageStream, err := cli.ImageSave(ctx, []string{imageName})
	if err != nil {
		return "", err
	}
	defer imageStream.Close()

	safeFilename := strings.ReplaceAll(imageName, ":", "_") + ".tar"
	targetPath := filepath.Join(exportDir, safeFilename)

	outFile, err := os.Create(targetPath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, imageStream)
	if err != nil {
		return "", err
	}

	return targetPath, nil
}