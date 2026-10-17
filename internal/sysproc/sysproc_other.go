//go:build !windows

// Package sysproc tweaks child processes per platform.
package sysproc

import (
	"os/exec"
	"syscall"
)

// Prepare puts the child in its own process group so it can be stopped
// together with anything it spawns.
func Prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// Terminate asks the whole process group to exit.
func Terminate(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
}

// Kill force-stops the whole process group.
func Kill(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
