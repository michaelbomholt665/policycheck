// internal/cliwrapper/chain.go
package cliwrapper

import (
	"context"
	"fmt"
)

// ThenToken is the delimiter that separates the gate command from the main
// command in a `-then` chain invocation.
const ThenToken = "-then"

// ExecFunc is the function signature used by RunChain to execute each command
// in a chain. Callers inject an ExecFunc to keep subprocess management
// outside core chain logic and to enable deterministic test doubles.
type ExecFunc func(ctx context.Context, args []string) error

// SplitChain splits args at the first occurrence of ThenToken.
//
// Returns (nil, args, false) when ThenToken is not present.
// Returns (gate, main, true) when the token is found; gate is the slice
// before the token, main is the slice after.
func SplitChain(args []string) (gate, main []string, chained bool) {
	for i, tok := range args {
		if tok == ThenToken {
			return args[:i], args[i+1:], true
		}
	}

	return nil, args, false
}

// RunChain executes the gate command first. If the gate succeeds it executes
// the main command. If the gate fails the main command is skipped and the gate
// error is returned (wrapped with chain context).
//
// Context cancellation is honoured before each exec call; the cancellation
// error is returned immediately without executing further commands.
func RunChain(ctx context.Context, gate, main []string, exec ExecFunc) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("chain: context cancelled before gate: %w", err)
	}

	if err := exec(ctx, gate); err != nil {
		return fmt.Errorf("chain: gate command failed: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("chain: context cancelled before main: %w", err)
	}

	if err := exec(ctx, main); err != nil {
		return fmt.Errorf("chain: main command failed: %w", err)
	}

	return nil
}
