package service

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/BGYKanishka/baleen-engine/internal/config"
)

// BackgroundEnvKey is the environment variable that distinguishes the real
// background daemon from the short-lived launcher process.
const BackgroundEnvKey = "BALEEN_BACKGROUND"

// returns true when the current process is the real background daemon,
// false when it's the short-lived launcher process.
func IsBackgroundProcess() bool {
	return os.Getenv(BackgroundEnvKey) == "1"
}

// returns the path to the daemon log file.
func daemonLogPath() (string, error) {
	baleenRoot, err := config.BaleenDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baleenRoot, "daemon.log"), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// LaunchBackground launches the background daemon process, which will run
// independently of the current process. The daemon will write its logs to a
// file in the user's home directory.
func LaunchBackground(token string, name string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	if runtime.GOOS == "windows" {
		baleenRoot, err := config.BaleenDir()
		if err == nil {
			binDir := filepath.Join(baleenRoot, "bin")
			os.MkdirAll(binDir, 0755)

			newExePath := filepath.Join(binDir, "baleen-daemon.exe")
			oldExePath := filepath.Join(binDir, "baleen-daemon.old")

			os.Remove(oldExePath)
			os.Rename(newExePath, oldExePath)

			if err := copyFile(exePath, newExePath); err == nil {
				exePath = newExePath
			}

			// Clean up leftover PID-based executables from previous versions
			if files, err := os.ReadDir(binDir); err == nil {
				for _, f := range files {
					name := f.Name()
					if name != "baleen-daemon.exe" && filepath.Ext(name) == ".exe" {
						os.Remove(filepath.Join(binDir, name))
					}
				}
			}
		}
	}

	logPath, err := daemonLogPath()
	if err != nil {
		return fmt.Errorf("resolve log path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return fmt.Errorf("create baleen directory: %w", err)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open daemon log: %w", err)
	}

	cmd := exec.Command(exePath, "daemon", "--token", token, "--name", name)
	cmd.Env = append(os.Environ(), BackgroundEnvKey+"=1")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil

	setProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start background process: %w", err)
	}
	logFile.Close()
	return nil
}
