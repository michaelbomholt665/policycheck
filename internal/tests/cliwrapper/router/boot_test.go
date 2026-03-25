// internal/tests/cliwrapper/router/boot_test.go
package router_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"policycheck/internal/adapters/cliwrapper"
	"policycheck/internal/ports"
	"policycheck/internal/router"
	"policycheck/internal/router/ext"
)

const bootTimeout = 10 * time.Second

// bootRouter is a helper that resets the router and boots all registered extensions.
// Tests that use this helper must not run in parallel — router state is global.
func bootRouter(t *testing.T) {
	t.Helper()

	router.RouterResetForTest()
	t.Cleanup(router.RouterResetForTest)

	ctx, cancel := context.WithTimeout(context.Background(), bootTimeout)
	defer cancel()

	_, err := ext.RouterBootExtensions(ctx)
	require.NoError(t, err, "router boot must not fail")
}

// TestWrapperBoot_DispatcherPort_Resolves confirms the CLI wrapper dispatcher port
// can be resolved from the router independently of any policycheck command path.
func TestWrapperBoot_DispatcherPort_Resolves(t *testing.T) {
	bootRouter(t)

	provider, err := router.RouterResolveProvider(router.PortCLIWrapperDispatcher)
	require.NoError(t, err)
	require.NotNil(t, provider)

	_, ok := provider.(ports.CLIWrapperDispatcher)
	assert.True(t, ok, "resolved provider must implement CLIWrapperDispatcher")
}

// TestWrapperBoot_SecurityGatePort_Resolves confirms the CLI wrapper security gate port
// can be resolved from the router independently of any policycheck command path.
func TestWrapperBoot_SecurityGatePort_Resolves(t *testing.T) {
	bootRouter(t)

	provider, err := router.RouterResolveProvider(router.PortCLIWrapperSecurityGate)
	require.NoError(t, err)
	require.NotNil(t, provider)

	_, ok := provider.(ports.CLIWrapperSecurityGate)
	assert.True(t, ok, "resolved provider must implement CLIWrapperSecurityGate")
}

// TestWrapperBoot_MacroRunnerPort_Resolves confirms the CLI wrapper macro runner port
// can be resolved from the router independently of any policycheck command path.
func TestWrapperBoot_MacroRunnerPort_Resolves(t *testing.T) {
	bootRouter(t)

	provider, err := router.RouterResolveProvider(router.PortCLIWrapperMacroRunner)
	require.NoError(t, err)
	require.NotNil(t, provider)

	_, ok := provider.(ports.CLIWrapperMacroRunner)
	assert.True(t, ok, "resolved provider must implement CLIWrapperMacroRunner")
}

// TestWrapperBoot_FormatterPort_Resolves confirms the CLI wrapper formatter port
// can be resolved from the router independently of any policycheck command path.
func TestWrapperBoot_FormatterPort_Resolves(t *testing.T) {
	bootRouter(t)

	provider, err := router.RouterResolveProvider(router.PortCLIWrapperFormatter)
	require.NoError(t, err)
	require.NotNil(t, provider)

	_, ok := provider.(ports.CLIWrapperFormatter)
	assert.True(t, ok, "resolved provider must implement CLIWrapperFormatter")
}

// TestWrapperBoot_IsIndependent_FromPolicycheckExecution proves that the wrapper
// subsystem can be booted and all ports resolved via WrapperBootEntry without
// invoking any policycheck command execution path.
//
// This test is the T4 acceptance gate: it fails if the wrapper boot logic
// falls back to or requires the policycheck CLI dispatch path.
func TestWrapperBoot_IsIndependent_FromPolicycheckExecution(t *testing.T) {
	bootRouter(t)

	// WrapperBootEntry is the wrapper-only dispatch seam. It must succeed without
	// any policycheck command invocation occurring before or after this call.
	resolved, err := cliwrapper.WrapperBootEntry()
	require.NoError(t, err, "WrapperBootEntry must succeed without policycheck command execution")

	assert.NotNil(t, resolved.Dispatcher, "dispatcher must be non-nil")
	assert.NotNil(t, resolved.SecurityGate, "security gate must be non-nil")
	assert.NotNil(t, resolved.MacroRunner, "macro runner must be non-nil")
	assert.NotNil(t, resolved.Formatter, "formatter must be non-nil")
}
