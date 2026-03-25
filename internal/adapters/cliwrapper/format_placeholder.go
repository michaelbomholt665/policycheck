// internal/adapters/cliwrapper/format_placeholder.go
package cliwrapper

import (
	"context"
	"fmt"
)

// FormatterPlaceholder is a placeholder implementation of ports.CLIWrapperFormatter.
//
// It returns a wrapped sentinel error on every call. Replace with real header-injection
// logic once the fmt walker and header detector are implemented.
type FormatterPlaceholder struct{}

// NewFormatterPlaceholder returns a new FormatterPlaceholder.
func NewFormatterPlaceholder() *FormatterPlaceholder {
	return &FormatterPlaceholder{}
}

// FormatHeaders always returns a wrapper-context not-implemented error.
func (p *FormatterPlaceholder) FormatHeaders(_ context.Context, _ bool, _ []string) error {
	return fmt.Errorf("cli wrapper formatter placeholder: %w", errNotImplemented)
}
