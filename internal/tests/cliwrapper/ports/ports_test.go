// internal/tests/cliwrapper/ports/ports_test.go
package ports_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"policycheck/internal/ports"
)

// TestCLIWrapperDispatcher_InterfaceExists confirms the CLIWrapperDispatcher interface
// is defined in the ports package and accepts the expected method signature.
func TestCLIWrapperDispatcher_InterfaceExists(t *testing.T) {
	t.Parallel()

	var _ ports.CLIWrapperDispatcher = dispatcherProxy{}
	assert.True(t, true, "CLIWrapperDispatcher interface satisfied at compile time")
}

// TestCLIWrapperSecurityGate_InterfaceExists confirms the CLIWrapperSecurityGate interface
// is defined in the ports package and accepts the expected method signature.
func TestCLIWrapperSecurityGate_InterfaceExists(t *testing.T) {
	t.Parallel()

	var _ ports.CLIWrapperSecurityGate = securityProxy{}
	assert.True(t, true, "CLIWrapperSecurityGate interface satisfied at compile time")
}

// TestCLIWrapperMacroRunner_InterfaceExists confirms the CLIWrapperMacroRunner interface
// is defined in the ports package and accepts the expected method signature.
func TestCLIWrapperMacroRunner_InterfaceExists(t *testing.T) {
	t.Parallel()

	var _ ports.CLIWrapperMacroRunner = macroProxy{}
	assert.True(t, true, "CLIWrapperMacroRunner interface satisfied at compile time")
}

// TestCLIWrapperFormatter_InterfaceExists confirms the CLIWrapperFormatter interface
// is defined in the ports package and accepts the expected method signature.
func TestCLIWrapperFormatter_InterfaceExists(t *testing.T) {
	t.Parallel()

	var _ ports.CLIWrapperFormatter = formatterProxy{}
	assert.True(t, true, "CLIWrapperFormatter interface satisfied at compile time")
}

// -- Minimal compile-time adapters (satisfy each interface without a real implementation) --

type dispatcherProxy struct{}

func (dispatcherProxy) Dispatch(_ context.Context, _ []string) error { return nil }

type securityProxy struct{}

func (securityProxy) CheckPackages(_ context.Context, _ string, _ []string) error { return nil }
func (securityProxy) CheckLockfile(_ context.Context, _ string, _ string) error   { return nil }

type macroProxy struct{}

func (macroProxy) RunMacro(_ context.Context, _ string) error { return nil }

type formatterProxy struct{}

func (formatterProxy) FormatHeaders(_ context.Context, _ bool, _ bool, _ []string) error {
	return nil
}
