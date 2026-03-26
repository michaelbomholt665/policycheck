//go:build windows

package cliwrapper

import (
	"fmt"
	"os/exec"
)

func configureManagedCommand(_ *exec.Cmd) {}

func terminateManagedCommand(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	if err := cmd.Process.Kill(); err != nil {
		return fmt.Errorf("kill process %d: %w", cmd.Process.Pid, err)
	}

	return nil
}
