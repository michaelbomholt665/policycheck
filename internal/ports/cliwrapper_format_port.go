// internal/ports/cliwrapper_format_port.go
// Declares the router port for wrapper-driven source header formatting.
// Keeps header-formatting consumers isolated from filesystem-specific implementations.
package ports

import "context"

// CLIWrapperFormatter is the formatting contract for the CLI wrapper subsystem.
//
// Implementations walk the repository and inject or correct path-comment headers
// in supported source files. This interface belongs to the CLI-wrapper subsystem
// and is not shared with the policycheck analysis engine.
type CLIWrapperFormatter interface {
	// FormatHeaders walks the repository and injects or corrects path-comment headers.
	//
	// When dryRun is true, no files are written; an error is returned if any file
	// would be modified (CI-safe mode). The only slice restricts formatting to the
	// named languages; an empty slice means all supported languages.
	FormatHeaders(ctx context.Context, dryRun bool, only []string) error
}
