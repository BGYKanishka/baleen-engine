package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func runInstallCLI() {
	// Find the current executable path
	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get executable path: %v\n", err)
		os.Exit(1)
	}

	// Resolve any symlinks to get the real path
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to resolve executable symlinks: %v\n", err)
		os.Exit(1)
	}

	// Find the user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get user home directory: %v\n", err)
		os.Exit(1)
	}

	// Determine the target directory for Docker CLI plugins
	targetDir := filepath.Join(homeDir, ".docker", "cli-plugins")
	if runtime.GOOS == "windows" {
		targetDir = filepath.Join(homeDir, ".docker", "cli-plugins")
	}

	// Ensure the target directory exists
	err = os.MkdirAll(targetDir, 0755)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create directory %s: %v\n", targetDir, err)
		os.Exit(1)
	}

	// Determine the target filename
	targetFileName := "docker-baleen"
	if runtime.GOOS == "windows" {
		targetFileName += ".exe"
	}
	targetPath := filepath.Join(targetDir, targetFileName)

	// Copy the executable
	err = copyFile(exePath, targetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to install CLI to %s: %v\n", targetPath, err)
		os.Exit(1)
	}

	// Make it executable
	err = os.Chmod(targetPath, 0755)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to make %s executable: %v\n", targetPath, err)
		os.Exit(1)
	}

	// Re-sign on macOS ARM64 to prevent SIGKILL (137)
	if runtime.GOOS == "darwin" {
		cmd := exec.Command("codesign", "-s", "-", targetPath)
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to codesign binary: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("Successfully installed CLI to %s\n", targetPath)
	fmt.Printf("You can now use 'docker baleen' in your terminal.\n")
}

func runCheckCLI() {
	// Find the user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get user home directory: %v\n", err)
		os.Exit(1)
	}

	// Determine the target directory for Docker CLI plugins
	targetDir := filepath.Join(homeDir, ".docker", "cli-plugins")
	if runtime.GOOS == "windows" {
		targetDir = filepath.Join(homeDir, ".docker", "cli-plugins")
	}

	// Determine the target filename
	targetFileName := "docker-baleen"
	if runtime.GOOS == "windows" {
		targetFileName += ".exe"
	}
	targetPath := filepath.Join(targetDir, targetFileName)

	// Check if the file exists
	info, err := os.Stat(targetPath)
	if err == nil && info.Size() > 0 {
		fmt.Printf("Installed\n")
		os.Exit(0)
	} else {
		fmt.Fprintf(os.Stderr, "Not installed\n")
		os.Exit(1)
	}
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
