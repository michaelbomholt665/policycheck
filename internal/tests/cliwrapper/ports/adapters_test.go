// internal/tests/cliwrapper/ports/adapters_test.go
package ports_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"policycheck/internal/adapters/cliwrapper"
	"policycheck/internal/ports"
)

// TestDispatcherPlaceholder_Dispatch_ReturnsNotImplementedError verifies the placeholder
// dispatcher returns a wrapper-context error and never panics on empty input.
func TestDispatcherPlaceholder_Dispatch_ReturnsNotImplementedError(t *testing.T) {
	t.Parallel()

	var p ports.CLIWrapperDispatcher = cliwrapper.NewDispatcherPlaceholder()
	err := p.Dispatch(context.Background(), []string{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cli wrapper dispatcher placeholder")
}

// TestDispatcherPlaceholder_Dispatch_NilArgs_DoesNotPanic confirms nil args input
// does not cause a panic.
func TestDispatcherPlaceholder_Dispatch_NilArgs_DoesNotPanic(t *testing.T) {
	t.Parallel()

	p := cliwrapper.NewDispatcherPlaceholder()
	assert.NotPanics(t, func() {
		_ = p.Dispatch(context.Background(), nil)
	})
}

// TestSecurityGatePlaceholder_CheckPackages_ReturnsNotImplementedError verifies the
// placeholder security gate returns a wrapper-context error.
func TestSecurityGatePlaceholder_CheckPackages_ReturnsNotImplementedError(t *testing.T) {
	t.Parallel()

	var p ports.CLIWrapperSecurityGate = cliwrapper.NewSecurityGatePlaceholder()
	err := p.CheckPackages(context.Background(), "", []string{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cli wrapper security gate placeholder")
}

// TestSecurityGatePlaceholder_CheckLockfile_ReturnsNotImplementedError verifies the
// placeholder lockfile scan path returns a wrapper-context error.
func TestSecurityGatePlaceholder_CheckLockfile_ReturnsNotImplementedError(t *testing.T) {
	t.Parallel()

	var p ports.CLIWrapperSecurityGate = cliwrapper.NewSecurityGatePlaceholder()
	err := p.CheckLockfile(context.Background(), "", "package-lock.json")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cli wrapper security gate placeholder")
}
