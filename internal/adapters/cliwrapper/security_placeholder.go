// internal/adapters/cliwrapper/security_placeholder.go
package cliwrapper

import (
	"context"
	"fmt"
)

// SecurityGatePlaceholder is a placeholder implementation of ports.CLIWrapperSecurityGate.
//
// It returns a wrapped sentinel error on every call. Replace with real OSV integration
// once the security gate layer is implemented.
type SecurityGatePlaceholder struct{}

// NewSecurityGatePlaceholder returns a new SecurityGatePlaceholder.
func NewSecurityGatePlaceholder() *SecurityGatePlaceholder {
	return &SecurityGatePlaceholder{}
}

// CheckPackages always returns a wrapper-context not-implemented error.
func (p *SecurityGatePlaceholder) CheckPackages(_ context.Context, _ string, _ []string) error {
	return fmt.Errorf("cli wrapper security gate placeholder: %w", errNotImplemented)
}
