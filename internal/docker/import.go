package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/docker/docker/api/types/image"
)

// reads a .tar file and loads it into the Docker
func (m *Manager) LoadImage(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	ctx := context.Background()

	resp, err := m.Cli.ImageLoad(ctx, file)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var loadedImage string
	decoder := json.NewDecoder(resp.Body)
	for {
		var msg struct {
			Stream string `json:"stream"`
			Error  string `json:"error"`
		}
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		if msg.Error != "" {
			return "", fmt.Errorf("docker load error: %s", msg.Error)
		}
		if strings.HasPrefix(msg.Stream, "Loaded image: ") {
			loadedImage = strings.TrimSpace(strings.TrimPrefix(msg.Stream, "Loaded image: "))
		} else if strings.HasPrefix(msg.Stream, "Loaded image ID: ") {
			loadedImage = strings.TrimSpace(strings.TrimPrefix(msg.Stream, "Loaded image ID: "))
		}
	}

	if loadedImage == "" {
		return "", fmt.Errorf("could not determine loaded image from docker load output")
	}

	return loadedImage, nil
}

// loads a Docker image from a .tar file and re-tags it with the specified image name.
func (m *Manager) LoadAndTag(filePath, imageName string) error {
	ctx := context.Background()

	// Check if the target image already exists to back it up BEFORE we load the new one
	if _, err := m.Cli.ImageInspect(ctx, imageName); err == nil {
		repo := imageName
		if idx := strings.LastIndex(imageName, ":"); idx != -1 && !strings.Contains(imageName[idx:], "/") {
			repo = imageName[:idx]
		}

		for i := 1; ; i++ {
			nextTag := fmt.Sprintf("%s:v%d", repo, i)
			if _, err := m.Cli.ImageInspect(ctx, nextTag); err != nil {
				if err := m.Cli.ImageTag(ctx, imageName, nextTag); err != nil {
					slog.Warn("failed to backup existing image", "image", imageName, "nextTag", nextTag, "error", err)
				} else {
					slog.Info("backed up existing image", "old", imageName, "new", nextTag)
				}
				break
			}
		}
	}

	// Load the image from the tarball
	loadedID, err := m.LoadImage(filePath)
	if err != nil {
		return err
	}

	// Always ensure the loaded image has the target imageName tag
	if loadedID != imageName {
		if err := m.Cli.ImageTag(ctx, loadedID, imageName); err != nil {
			return fmt.Errorf("failed to tag loaded image %s to %s: %w", loadedID, imageName, err)
		}
	}

	// If a temporary cross-compile tag was used, remove it to clean up
	tmpName := imageName + "-baleen-tmp"
	if loadedID == tmpName {
		if _, err := m.Cli.ImageRemove(ctx, tmpName, image.RemoveOptions{Force: true, PruneChildren: false}); err != nil {
			slog.Error("failed to remove temporary tag", "tag", tmpName, "error", err)
		}
	}

	return nil
}
