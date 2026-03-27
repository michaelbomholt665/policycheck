//go:build windows

// internal/adapters/cliwrapper/exec_windows.go
package cliwrapper

import (
	"fmt"
	"os/exec"
)

func configureManagedCommand(_ *exec.Cmd) {
	// Managed command configuration is a no-op on Windows.
	// Windows process groups (job objects) are handled differently than POSIX pgids.
}

func terminateManagedCommand(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	if err := cmd.Process.Kill(); err != nil {
		return fmt.Errorf("kill process %d: %w", cmd.Process.Pid, err)
	}

	return nil
}
