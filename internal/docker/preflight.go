package docker

import (
	"context"
	"fmt"
	"strings"
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

func archOnly(platform string) string {
	parts := strings.Split(platform, "/")
	return parts[len(parts)-1]
}

func (m *Manager) RunPreflightHandshake(config PreflightConfig) HandshakeReport {
	report := HandshakeReport{
		Passed:         true,
		TargetPlatform: config.ExpectedTarget,
	}

	ctx := context.Background()
	inspectData, err := m.Cli.ImageInspect(ctx, config.ImageName)

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
