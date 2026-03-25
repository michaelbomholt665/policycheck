// internal/ports/cliwrapper_security_port.go
package ports

import "context"

// CLIWrapperSecurityGate is the security-gate contract for the CLI wrapper subsystem.
//
// Implementations query vulnerability databases (e.g. OSV) for the supplied purls
// and return a blocking error when the configured severity threshold is exceeded.
// This interface belongs to the CLI-wrapper subsystem; it is distinct from any
// policycheck analysis-engine security concepts.
type CLIWrapperSecurityGate interface {
	// CheckPackages queries the security backend for each purl in the given ecosystem.
	// Returns a non-nil error if any package exceeds the configured block threshold.
	CheckPackages(ctx context.Context, ecosystem string, purls []string) error
}
