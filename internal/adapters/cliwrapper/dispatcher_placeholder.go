// internal/adapters/cliwrapper/dispatcher_placeholder.go
package cliwrapper

import (
	"context"
	"errors"
	"fmt"
)

// errNotImplemented is a package-level sentinel for placeholder adapters that have
// not yet been replaced with real business logic.
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
