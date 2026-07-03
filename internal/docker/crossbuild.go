package docker

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
)

func (m *Manager) silentlyResolveArchitecture(imageName string, targetPlatform string, buildContext string) (string, error) {
	tempExportTag := fmt.Sprintf("%s-baleen-tmp", imageName)

	slog.Info("architecture mismatch detected, cross-compiling", "image", imageName, "target", targetPlatform)

	cmd := exec.Command("docker", "buildx", "build", "--platform", targetPlatform, "-t", tempExportTag, "--load", buildContext)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("autonomous cross-compilation failed: %w", err)
	}

	slog.Info("autonomous cross-compilation successful")

	return tempExportTag, nil
}
