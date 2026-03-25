// internal/adapters/cliwrapper/macro_placeholder.go
package cliwrapper

import (
	"context"
	"fmt"
)

// MacroRunnerPlaceholder is a placeholder implementation of ports.CLIWrapperMacroRunner.
//
// It returns a wrapped sentinel error on every call. Replace with real macro runner logic
// once the macro config layer and step executor are implemented.
type MacroRunnerPlaceholder struct{}

// NewMacroRunnerPlaceholder returns a new MacroRunnerPlaceholder.
func NewMacroRunnerPlaceholder() *MacroRunnerPlaceholder {
	return &MacroRunnerPlaceholder{}
}

// RunMacro always returns a wrapper-context not-implemented error.
func (p *MacroRunnerPlaceholder) RunMacro(_ context.Context, _ string) error {
	return fmt.Errorf("cli wrapper macro runner placeholder: %w", errNotImplemented)
}
