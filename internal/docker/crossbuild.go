package docker

import (
	"fmt"
	"os"
	"os/exec"
)

func silentlyResolveArchitecture(imageName string, targetPlatform string, buildContext string) (string, error) {
	tempExportTag := fmt.Sprintf("%s-baleen-tmp", imageName)

	fmt.Printf("\nArchitecture mismatch detected. Cross-compiling %s for %s locally...\n", imageName, targetPlatform)

	cmd := exec.Command("docker", "buildx", "build", "--platform", targetPlatform, "-t", tempExportTag, "--load", buildContext)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("autonomous cross-compilation failed: %w", err)
	}

	fmt.Printf("\nAutonomous cross-compilation successful.\n")

	return tempExportTag, nil
}
