//go:build !windows

// internal/adapters/cliwrapper/exec_unix.go
package cliwrapper

import (
	"fmt"
	"os/exec"
	"syscall"
)

func configureManagedCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminateManagedCommand(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		return fmt.Errorf("kill process group for pid %d: %w", cmd.Process.Pid, err)
	}

	return nil
}
