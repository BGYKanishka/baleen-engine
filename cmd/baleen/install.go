package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BGYKanishka/baleen-engine/internal/service"
)

func SelfCleanupIfExtensionRemoved() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	pluginDir := filepath.Join(homeDir, ".docker", "cli-plugins")

	if !strings.HasPrefix(exePath, pluginDir) {
		return
	}

	// Check if Docker CLI is available.
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return // Can't verify — skip cleanup.
	}

	// Run `docker extension ls` to see if the baleen extension is still installed.
	cmd := exec.Command(dockerPath, "extension", "ls")
	output, err := cmd.Output()
	if err != nil {
		return
	}
	if strings.Contains(strings.ToLower(string(output)), "baleen") {
		return
	}

	// Stop the daemon if it's running.
	if existing, err := service.ReadState(); err == nil && service.IsAlive(existing) {
		stopURL := fmt.Sprintf("http://127.0.0.1:%d/api/stop", existing.Port)
		req, err := http.NewRequest("POST", stopURL, nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+existing.Token)
			client := &http.Client{}
			client.Do(req)
		}
	}

	// Clear the service state file.
	service.ClearState()

	// Remove the CLI plugin binary.
	targetFileName := "docker-baleen"
	if runtime.GOOS == "windows" {
		targetFileName += ".exe"
	}
	targetPath := filepath.Join(pluginDir, targetFileName)

	if runtime.GOOS == "windows" {

		os.Rename(targetPath, targetPath+".removed")
	} else {
		os.Remove(targetPath)
	}

	fmt.Println("Baleen extension has been uninstalled. CLI plugin removed.")
	os.Exit(0)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func AutoUpdateCLIPlugin() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	targetDir := filepath.Join(homeDir, ".docker", "cli-plugins")
	targetFileName := "docker-baleen"
	if runtime.GOOS == "windows" {
		targetFileName += ".exe"
	}
	targetPath := filepath.Join(targetDir, targetFileName)

	if exePath == targetPath {
		return
	}

	srcInfo, err1 := os.Stat(exePath)
	dstInfo, err2 := os.Stat(targetPath)

	if err1 == nil && err2 == nil {
		if dstInfo.Size() > 0 && srcInfo.Size() == dstInfo.Size() {
			return
		}
	}

	os.MkdirAll(targetDir, 0755)

	if runtime.GOOS == "windows" {
		os.Remove(targetPath + ".old")
		os.Rename(targetPath, targetPath+".old")
	}

	if err := copyFile(exePath, targetPath); err == nil {
		os.Chmod(targetPath, 0755)
		if runtime.GOOS == "darwin" {
			exec.Command("codesign", "-s", "-", targetPath).Run()
		}
	}
}
