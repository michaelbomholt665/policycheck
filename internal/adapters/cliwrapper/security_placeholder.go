// internal/adapters/cliwrapper/security_placeholder.go
package cliwrapper

import (
	"context"
	"errors"
	"fmt"
)

// errNotImplemented is a package-level sentinel for placeholder adapters that
// have not yet been replaced with real wrapper implementations.
var errNotImplemented = errors.New("not implemented")

// DispatcherPlaceholder is a placeholder implementation of ports.CLIWrapperDispatcher.
//
// It returns a wrapped sentinel error on every call. Replace with real dispatch logic
// once the wrapper command classification layer is implemented.
type DispatcherPlaceholder struct{}

// NewDispatcherPlaceholder returns a new DispatcherPlaceholder.
func NewDispatcherPlaceholder() *DispatcherPlaceholder {
	return &DispatcherPlaceholder{}
}

// Dispatch always returns a wrapper-context not-implemented error.
func (p *DispatcherPlaceholder) Dispatch(_ context.Context, _ []string) error {
	return fmt.Errorf("cli wrapper dispatcher placeholder: %w", errNotImplemented)
}

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

// CheckLockfile always returns a wrapper-context not-implemented error.
func (p *SecurityGatePlaceholder) CheckLockfile(_ context.Context, _ string, _ string) error {
	return fmt.Errorf("cli wrapper security gate placeholder: %w", errNotImplemented)
}
