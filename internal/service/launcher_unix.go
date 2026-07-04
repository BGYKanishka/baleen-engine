//go:build !windows

package service

import (
	"os/exec"
	"syscall"
)

// sets the process attributes for a command to run in a new session, detached from the console.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
}
