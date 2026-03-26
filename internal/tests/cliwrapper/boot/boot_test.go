// internal/tests/cliwrapper/boot/boot_test.go
package boot_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"policycheck/internal/cliwrapper"
	"policycheck/internal/ports"
	"policycheck/internal/router"
	"policycheck/internal/router/ext"
)

const bootTimeout = 10 * time.Second

// bootRouter resets and boots the router. Tests using this helper must NOT run
// in parallel — router state is global.
func bootRouter(t *testing.T) {
	t.Helper()

	router.RouterResetForTest()
	t.Cleanup(router.RouterResetForTest)

	ctx, cancel := context.WithTimeout(context.Background(), bootTimeout)
	defer cancel()

	_, err := ext.RouterBootExtensions(ctx)
	require.NoError(t, err, "router boot must not fail")
}

// TestWrapperBoot_DispatcherPort_Resolves confirms the dispatcher port is
// reachable through the router after boot.
func TestWrapperBoot_DispatcherPort_Resolves(t *testing.T) {
	bootRouter(t)

	provider, err := router.RouterResolveProvider(router.PortCLIWrapperDispatcher)
	require.NoError(t, err)
	require.NotNil(t, provider)

	_, ok := provider.(ports.CLIWrapperDispatcher)
	assert.True(t, ok, "resolved provider must implement CLIWrapperDispatcher")
}

// TestWrapperBoot_SecurityGatePort_Resolves confirms the security gate port
// resolves correctly after boot.
func TestWrapperBoot_SecurityGatePort_Resolves(t *testing.T) {
	bootRouter(t)

	provider, err := router.RouterResolveProvider(router.PortCLIWrapperSecurityGate)
	require.NoError(t, err)
	require.NotNil(t, provider)

	_, ok := provider.(ports.CLIWrapperSecurityGate)
	assert.True(t, ok, "resolved provider must implement CLIWrapperSecurityGate")
}

// TestWrapperBoot_MacroRunnerPort_Resolves confirms the macro runner port
// resolves correctly after boot.
func TestWrapperBoot_MacroRunnerPort_Resolves(t *testing.T) {
	bootRouter(t)

	provider, err := router.RouterResolveProvider(router.PortCLIWrapperMacroRunner)
	require.NoError(t, err)
	require.NotNil(t, provider)

	_, ok := provider.(ports.CLIWrapperMacroRunner)
	assert.True(t, ok, "resolved provider must implement CLIWrapperMacroRunner")
}

// TestWrapperBoot_FormatterPort_Resolves confirms the formatter port resolves
// correctly after boot.
func TestWrapperBoot_FormatterPort_Resolves(t *testing.T) {
	bootRouter(t)

	provider, err := router.RouterResolveProvider(router.PortCLIWrapperFormatter)
	require.NoError(t, err)
	require.NotNil(t, provider)

	_, ok := provider.(ports.CLIWrapperFormatter)
	assert.True(t, ok, "resolved provider must implement CLIWrapperFormatter")
}

// TestWrapperBoot_IsIndependent_FromPolicycheckExecution is the T4 acceptance
// gate: WrapperBootEntry must succeed without invoking any policycheck command
// execution path.
func TestWrapperBoot_IsIndependent_FromPolicycheckExecution(t *testing.T) {
	bootRouter(t)

	resolved, err := cliwrapper.WrapperBootEntry()
	require.NoError(t, err, "WrapperBootEntry must succeed without policycheck command execution")

	assert.NotNil(t, resolved.Dispatcher, "dispatcher must be non-nil")
	assert.NotNil(t, resolved.SecurityGate, "security gate must be non-nil")
	assert.NotNil(t, resolved.MacroRunner, "macro runner must be non-nil")
	assert.NotNil(t, resolved.Formatter, "formatter must be non-nil")
}
