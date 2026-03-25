// internal/ports/cliwrapper_macro_port.go
package ports

import "context"

// CLIWrapperMacroRunner is the macro execution contract for the CLI wrapper subsystem.
//
// Implementations look up a named macro from the active config, execute each step
// in sequence, and respect on_failure semantics. This interface belongs to the
// CLI-wrapper subsystem and is not shared with the policycheck analysis engine.
type CLIWrapperMacroRunner interface {
	// RunMacro looks up the macro with the given name and executes its steps.
	// Returns an error if the macro is unknown, a step fails with stop semantics,
	// or process cleanup cannot complete.
	RunMacro(ctx context.Context, name string) error
}
