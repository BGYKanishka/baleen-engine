package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/client"
)

// holds the parameters for the handshake
type PreflightConfig struct {
	ImageName      string
	ExpectedTarget string
	ExportDir      string
	BuildContext   string
	ForceRawExport bool
}

// acts as an execution plan for the main engine
type HandshakeReport struct {
	Passed             bool
	RequiresCrossBuild bool
	TargetPlatform     string
	ActualPlatform     string
	FatalErrors        []error
}

// is the main entry point
func ExportImage(config PreflightConfig) (string, string, error) {
	fmt.Printf("Running pre-flight architecture checks for %s...\n", config.ImageName)

	report := runPreflightHandshake(config)
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
			resolvedImage, err := silentlyResolveArchitecture(config.ImageName, report.TargetPlatform, config.BuildContext)
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
	tarballPath, err := saveToTarball(imageToExport, config.ExportDir)

	if isTempImage {
		fmt.Printf("Engine Cleanup: Removing temporary cross-compiled image (%s)...\n", imageToExport)
		exec.Command("docker", "rmi", imageToExport).Run()
	}

	return tarballPath, finalArch, err
}

func GetImageLayers(imageName string) ([]string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	ctx := context.Background()
	inspectData, _, err := cli.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		return nil, err
	}

	return inspectData.RootFS.Layers, nil
}
func archOnly(platform string) string {
	parts := strings.Split(platform, "/")
	return parts[len(parts)-1]
}

func runPreflightHandshake(config PreflightConfig) HandshakeReport {
	report := HandshakeReport{
		Passed:         true,
		TargetPlatform: config.ExpectedTarget,
	}

	// Connect to Docker to inspect the local image
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		report.Passed = false
		report.FatalErrors = append(report.FatalErrors, fmt.Errorf("docker daemon unreachable: %w", err))
		return report
	}
	defer cli.Close()

	ctx := context.Background()
	inspectData, _, err := cli.ImageInspectWithRaw(ctx, config.ImageName)

	if err != nil {
		// Image doesn't exist locally — will need a cross-build
		report.RequiresCrossBuild = true
		report.ActualPlatform = "unknown"
	} else {
		actualPlatform := inspectData.Os + "/" + inspectData.Architecture
		report.ActualPlatform = actualPlatform
		// Check for explicit architecture match
		if archOnly(actualPlatform) != archOnly(config.ExpectedTarget) {
			report.RequiresCrossBuild = true
			fmt.Printf(
				"Architecture mismatch: image is %s, target needs %s. Will cross-compile.\n",
				actualPlatform, config.ExpectedTarget,
			)
		} else {
			fmt.Printf(
				"Architecture match: image %s is compatible with target %s.\n",
				actualPlatform, config.ExpectedTarget,
			)
		}
	}

	return report
}

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

// saves the specified image as a tarball in the export directory, returning the path to the tarball
func saveToTarball(imageName string, exportDir string) (string, error) {
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
