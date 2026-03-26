// internal/ports/cliwrapper_dispatcher_port.go
// Declares the router port for wrapper command dispatch orchestration.
// Keeps wrapper dispatch consumers bound to contracts instead of concrete adapters.
package ports

import "context"

// CLIWrapperDispatcher is the dispatch contract for the CLI wrapper subsystem.
//
// Implementations interpret the raw CLI args slice and route to the appropriate
// wrapper capability (security gate, macro runner, formatter, or passthrough).
// This interface belongs to the CLI-wrapper subsystem; it must not be reused for
// policycheck analysis engine commands.
type CLIWrapperDispatcher interface {
	// Dispatch interprets args and delegates to the matched wrapper capability.
	// Returns an error if dispatch fails or the matched capability is unavailable.
	Dispatch(ctx context.Context, args []string) error
}
