//go:build windows

package service

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	modKernel32            = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess        = modKernel32.NewProc("OpenProcess")
	procGetExitCodeProcess = modKernel32.NewProc("GetExitCodeProcess")
)

const (
	processQueryLimitedInformation = 0x1000
	stillActive                    = 259
)

// isProcessAlive checks if a given process is still running on Windows by querying its exit code.
// It returns true if the process is alive, false otherwise.
func isProcessAlive(proc *os.Process) bool {
	handle, _, err := procOpenProcess.Call(
		uintptr(processQueryLimitedInformation),
		0,
		uintptr(proc.Pid),
	)
	if handle == 0 || err != nil && err.(syscall.Errno) != 0 {
		return false
	}
	defer syscall.CloseHandle(syscall.Handle(handle))

	var code uint32
	ret, _, _ := procGetExitCodeProcess.Call(handle, uintptr(unsafe.Pointer(&code)))
	return ret != 0 && code == stillActive
}
