//go:build windows

// Package sysproc tweaks child processes per platform.
package sysproc

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// Prepare hides the console window of the child process.
func Prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

func Terminate(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func Kill(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
