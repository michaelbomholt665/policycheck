// internal/adapters/cliwrapper/exec.go
package cliwrapper

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"policycheck/internal/ports"
)

// CommandExitError distinguishes a child-process failure from wrapper-side
// orchestration failures such as parse, routing, or cancellation errors.
type CommandExitError struct {
	Command  string
	ExitCode int
	Err      error
}

// Error returns the stable child-process failure message.
func (e *CommandExitError) Error() string {
	if e == nil {
		return ""
	}

	return fmt.Sprintf("command %q exited with code %d", e.Command, e.ExitCode)
}

// Unwrap returns the underlying exec error.
func (e *CommandExitError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

// OsExec is the production exec-function implementation.
//
// OsExec runs args[0] with args[1:] as arguments, inheriting the current
// process environment and stdio streams. It is injected into WrapperDispatcher
// by the extension wiring; tests use their own exec-function double.
func OsExec(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("os exec: empty args")
	}

	cmd := newManagedCommand(ctx, args[0], args[1:]...)
	cmd.Stdout = nil // inherit parent stdout
	cmd.Stderr = nil // inherit parent stderr
	cmd.Stdin = nil  // inherit parent stdin

	// exec.Cmd.Stdout/Stderr nil means inherit the parent process file descriptors.
	// Use CombinedOutput would capture and mute output; we want it preserved.
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("os exec %q cancelled: %w", args[0], ctxErr)
		}

		return fmt.Errorf("os exec %q: %w", args[0], wrapCommandExitError(args[0], err))
	}

	return nil
}

func newManagedCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // wrapper executes caller-selected tooling
	configureManagedCommand(cmd)
	cmd.Cancel = func() error {
		return terminateManagedCommand(cmd)
	}
	cmd.WaitDelay = 2 * time.Second

	return cmd
}

func wrapCommandExitError(commandName string, err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return &CommandExitError{
			Command:  commandName,
			ExitCode: exitErr.ExitCode(),
			Err:      err,
		}
	}

	return err
}

// defaultExecFunc is the exec function the extension uses when building production
// WrapperDispatcher instances.
var defaultExecFunc ports.ExecFunc = OsExec
