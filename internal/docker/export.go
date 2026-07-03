package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/image"
)

// is the main entry point
func (m *Manager) ExportImage(config PreflightConfig) (string, string, error) {
	fmt.Printf("Running pre-flight architecture checks for %s...\n", config.ImageName)

	report := m.RunPreflightHandshake(config)
	if !report.Passed {
		return "", "", fmt.Errorf("export aborted due to fatal errors: %v", report.FatalErrors)
	}

	imageToExport := config.ImageName
	isTempImage := false
	finalArch := report.ActualPlatform

	// Autonomous Architecture Resolution
	if report.RequiresCrossBuild && !config.ForceRawExport {
		dockerfilePath := filepath.Join(config.BuildContext, "Dockerfile")
		if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
			fmt.Println("No Dockerfile found. Falling back to raw export (receiver will use emulation).")
			config.ForceRawExport = true
		} else {
			resolvedImage, err := m.silentlyResolveArchitecture(config.ImageName, report.TargetPlatform, config.BuildContext)
			if err != nil {
				return "", "", fmt.Errorf("failed to resolve architecture: %w", err)
			}
			imageToExport = resolvedImage
			isTempImage = true
			finalArch = report.TargetPlatform
		}
	}

	if config.ForceRawExport {
		fmt.Printf("Forcing Raw Export. Sending '%s' natively...\n", finalArch)
	}

	fmt.Printf("Exporting %s to tarball...\n", imageToExport)
	tarballPath, err := m.saveToTarball(imageToExport, config.ExportDir)

	if isTempImage {
		fmt.Printf("Engine Cleanup: Removing temporary cross-compiled image (%s)...\n", imageToExport)
		_, cleanupErr := m.Cli.ImageRemove(context.Background(), imageToExport, image.RemoveOptions{Force: true, PruneChildren: true})
		if cleanupErr != nil {
			fmt.Printf("Warning: failed to remove temporary cross-compiled image (%s): %v\n", imageToExport, cleanupErr)
		}
	}

	return tarballPath, finalArch, err
}

func (m *Manager) GetImageLayers(imageName string) ([]string, error) {
	ctx := context.Background()
	inspectData, _, err := m.Cli.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		return nil, err
	}

	return inspectData.RootFS.Layers, nil
}

// saves the specified image as a tarball in the export directory, returning the path to the tarball
func (m *Manager) saveToTarball(imageName string, exportDir string) (string, error) {
	ctx := context.Background()
	imageStream, err := m.Cli.ImageSave(ctx, []string{imageName})
	if err != nil {
		return "", err
	}
	defer imageStream.Close()

	safeFilename := strings.ReplaceAll(imageName, ":", "_")
	safeFilename = strings.ReplaceAll(safeFilename, "/", "-") + ".tar"
	targetPath := filepath.Join(exportDir, safeFilename)

	outFile, err := os.Create(targetPath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	if _, err = io.Copy(outFile, imageStream); err != nil {
		return "", err
	}

	return targetPath, nil
}
