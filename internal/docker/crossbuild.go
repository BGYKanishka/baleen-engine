package docker

import (
	"bytes"
	"fmt"
	"log/slog"
	"os/exec"
)

func (m *Manager) silentlyResolveArchitecture(imageName string, targetPlatform string, buildContext string) (string, error) {
	tempExportTag := fmt.Sprintf("%s-baleen-tmp", imageName)

	slog.Info("architecture mismatch detected, cross-compiling", "image", imageName, "target", targetPlatform)

	// Added --progress=quiet to hide noisy BuildKit output
	cmd := exec.Command("docker", "buildx", "build", "--progress=quiet", "--platform", targetPlatform, "-t", tempExportTag, "--load", buildContext)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		// Only show the Docker output if it fails!
		slog.Error("cross-compilation failed", "details", out.String())
		return "", fmt.Errorf("autonomous cross-compilation failed: %w", err)
	}

	slog.Info("autonomous cross-compilation successful")

	return tempExportTag, nil
}
