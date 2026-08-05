package docker

import (
	"bytes"
	"fmt"
	"log/slog"
	"os/exec"
	"sync"
)

type lineWriter struct {
	isErr bool
	buf   []byte
	mu    sync.Mutex
}

func (w *lineWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf = append(w.buf, p...)
	for {
		idx := bytes.IndexAny(w.buf, "\r\n")
		if idx == -1 {
			break
		}
		line := w.buf[:idx]
		w.buf = w.buf[idx+1:] // Skip the \r or \n
		if len(line) > 0 {
			if w.isErr {
				slog.Warn(string(line))
			} else {
				slog.Info(string(line))
			}
		}
	}
	return len(p), nil
}

func (m *Manager) silentlyResolveArchitecture(imageName string, targetPlatform string, buildContext string) (string, error) {
	tempExportTag := fmt.Sprintf("%s-baleen-tmp", imageName)

	slog.Info("architecture mismatch detected, cross-compiling", "image", imageName, "target", targetPlatform)

	cmd := exec.Command("docker", "buildx", "build", "--progress=plain", "--no-cache", "--platform", targetPlatform, "-t", tempExportTag, "--load", buildContext)

	cmd.Stdout = &lineWriter{isErr: false}
	cmd.Stderr = &lineWriter{isErr: true}

	if err := cmd.Start(); err != nil {
		slog.Error("failed to start cross-compilation", "error", err)
		return "", fmt.Errorf("autonomous cross-compilation failed: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		slog.Error("cross-compilation failed", "error", err)
		return "", fmt.Errorf("autonomous cross-compilation failed: %w", err)
	}

	slog.Info("autonomous cross-compilation successful")

	return tempExportTag, nil
}
