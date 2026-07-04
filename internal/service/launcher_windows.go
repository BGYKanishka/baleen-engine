//go:build windows

package service

import (
	"os/exec"
	"syscall"
)

// Windows process creation flags.
const (
	createNewProcessGroup = 0x00000200
	detachedProcess       = 0x00000008
)

// sets the process attributes for a command to run in a new process group and detached from the console.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNewProcessGroup | detachedProcess,
	}
}
