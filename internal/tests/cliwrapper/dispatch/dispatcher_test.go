// internal/tests/cliwrapper/dispatch/dispatcher_test.go
package dispatch_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cliwrapperadapter "policycheck/internal/adapters/cliwrapper"
	cliwrappercore "policycheck/internal/adapters/cliwrappercore"
	"policycheck/internal/ports"
)

// stubSecurityGate is a test double for ports.CLIWrapperSecurityGate.
type stubSecurityGate struct {
	packageErr  error
	lockfileErr error
	lockfiles   []string
}

func (s *stubSecurityGate) CheckPackages(_ context.Context, _ string, _ []string) error {
	return s.packageErr
}

func (s *stubSecurityGate) CheckLockfile(_ context.Context, _ string, lockfilePath string) error {
	s.lockfiles = append(s.lockfiles, lockfilePath)
	return s.lockfileErr
}

type stubMacroRunner struct {
	name string
	err  error
}

func (s *stubMacroRunner) RunMacro(_ context.Context, name string) error {
	s.name = name
	return s.err
}

type stubFormatter struct {
	dryRun bool
	only   []string
	err    error
}

func (s *stubFormatter) FormatHeaders(_ context.Context, dryRun bool, only []string) error {
	s.dryRun = dryRun
	s.only = append([]string(nil), only...)
	return s.err
}

// execRecorder records exec calls made by the dispatcher.
type execRecorder struct {
	calls [][]string
}

func (r *execRecorder) exec(_ context.Context, args []string) error {
	r.calls = append(r.calls, args)
	return nil
}

func stubCoreResolver() func() (ports.CLIWrapperCore, error) {
	return func() (ports.CLIWrapperCore, error) {
		return cliwrappercore.Provider{}, nil
	}
}

// TestDispatcher_PackageGate_Allowed verifies a package install is dispatched
// when the security gate allows it.
func TestDispatcher_PackageGate_Allowed(t *testing.T) {
	t.Parallel()

	gate := &stubSecurityGate{}
	rec := &execRecorder{}

	d := cliwrapperadapter.NewWrapperDispatcherWithResolvers(
		cliwrapperadapter.WrapperConfig{},
		rec.exec,
		cliwrapperadapter.WrapperResolvers{
			Core:         stubCoreResolver(),
			SecurityGate: func() (ports.CLIWrapperSecurityGate, error) { return gate, nil },
		},
	)
	err := d.Dispatch(context.Background(), []string{"npm", "install", "lodash"})

	require.NoError(t, err)
	assert.Equal(t, []string{"package-lock.json"}, gate.lockfiles)
	require.Len(t, rec.calls, 1)
}

// TestDispatcher_PackageGate_Blocked verifies a package install is stopped
// when the security gate blocks it.
func TestDispatcher_PackageGate_Blocked(t *testing.T) {
	t.Parallel()

	blockErr := &cliwrapperadapter.RiskBlockError{
		Severity: cliwrapperadapter.SeverityCritical,
		Reason:   "critical vulnerability in lodash",
	}
	gate := &stubSecurityGate{packageErr: blockErr}
	rec := &execRecorder{}

	d := cliwrapperadapter.NewWrapperDispatcherWithResolvers(
		cliwrapperadapter.WrapperConfig{},
		rec.exec,
		cliwrapperadapter.WrapperResolvers{
			Core:         stubCoreResolver(),
			SecurityGate: func() (ports.CLIWrapperSecurityGate, error) { return gate, nil },
		},
	)
	err := d.Dispatch(context.Background(), []string{"npm", "install", "lodash"})

	require.Error(t, err)
	assert.ErrorIs(t, err, blockErr)
	assert.Empty(t, rec.calls, "exec must not be called when gate blocks")
}

func TestDispatcher_PackageGate_AllowRiskOverridesMatchingSeverity(t *testing.T) {
	t.Parallel()

	gate := &stubSecurityGate{
		packageErr: &cliwrapperadapter.RiskBlockError{
			Severity: cliwrapperadapter.SeverityHigh,
			Reason:   "high vulnerability in lodash",
		},
	}
	rec := &execRecorder{}

	d := cliwrapperadapter.NewWrapperDispatcherWithResolvers(
		cliwrapperadapter.WrapperConfig{},
		rec.exec,
		cliwrapperadapter.WrapperResolvers{
			Core:         stubCoreResolver(),
			SecurityGate: func() (ports.CLIWrapperSecurityGate, error) { return gate, nil },
		},
	)
	err := d.Dispatch(context.Background(), []string{"npm", "install", "lodash", "--allow-risk=high"})

	require.NoError(t, err)
	require.Len(t, rec.calls, 1)
	assert.Equal(t, []string{"npm", "install", "lodash"}, rec.calls[0])
}

func TestDispatcher_PackageGate_AllowRiskTooLowStillBlocks(t *testing.T) {
	t.Parallel()

	gate := &stubSecurityGate{
		packageErr: &cliwrapperadapter.RiskBlockError{
			Severity: cliwrapperadapter.SeverityCritical,
			Reason:   "critical vulnerability in lodash",
		},
	}
	rec := &execRecorder{}

	d := cliwrapperadapter.NewWrapperDispatcherWithResolvers(
		cliwrapperadapter.WrapperConfig{},
		rec.exec,
		cliwrapperadapter.WrapperResolvers{
			Core:         stubCoreResolver(),
			SecurityGate: func() (ports.CLIWrapperSecurityGate, error) { return gate, nil },
		},
	)
	err := d.Dispatch(context.Background(), []string{"npm", "install", "lodash", "--allow-risk=high"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--allow-risk=high is insufficient")
	assert.Empty(t, rec.calls)
}

// TestDispatcher_PackageGate_PostInstallBlocked verifies a post-install lockfile
// scan still blocks the wrapper result after the package manager command runs.
func TestDispatcher_PackageGate_PostInstallBlocked(t *testing.T) {
	t.Parallel()

	blockErr := &cliwrapperadapter.RiskBlockError{
		Severity: cliwrapperadapter.SeverityHigh,
		Reason:   "transitive vulnerability in package-lock.json",
	}
	gate := &stubSecurityGate{lockfileErr: blockErr}
	rec := &execRecorder{}

	d := cliwrapperadapter.NewWrapperDispatcherWithResolvers(
		cliwrapperadapter.WrapperConfig{},
		rec.exec,
		cliwrapperadapter.WrapperResolvers{
			Core:         stubCoreResolver(),
			SecurityGate: func() (ports.CLIWrapperSecurityGate, error) { return gate, nil },
		},
	)
	err := d.Dispatch(context.Background(), []string{"npm", "install", "lodash"})

	require.Error(t, err)
	assert.ErrorIs(t, err, blockErr)
	require.Len(t, rec.calls, 1, "install command must run before post-install scan")
	assert.Equal(t, []string{"package-lock.json"}, gate.lockfiles)
}

// TestDispatcher_Passthrough verifies that args not matching any wrapper mode
// are forwarded directly to exec.
func TestDispatcher_Passthrough(t *testing.T) {
	t.Parallel()

	rec := &execRecorder{}

	d := cliwrapperadapter.NewWrapperDispatcherWithResolvers(
		cliwrapperadapter.WrapperConfig{},
		rec.exec,
		cliwrapperadapter.WrapperResolvers{Core: stubCoreResolver()},
	)
	err := d.Dispatch(context.Background(), []string{"echo", "hello"})

	require.NoError(t, err)
	require.Len(t, rec.calls, 1)
	assert.Equal(t, []string{"echo", "hello"}, rec.calls[0])
}

// TestDispatcher_ToolingChain verifies that -then chains execute in order.
func TestDispatcher_ToolingChain(t *testing.T) {
	t.Parallel()

	rec := &execRecorder{}

	d := cliwrapperadapter.NewWrapperDispatcherWithResolvers(
		cliwrapperadapter.WrapperConfig{},
		rec.exec,
		cliwrapperadapter.WrapperResolvers{Core: stubCoreResolver()},
	)
	err := d.Dispatch(context.Background(), []string{"go", "build", "./...", "-then", "go", "test", "./..."})

	require.NoError(t, err)
	require.Len(t, rec.calls, 2, "gate and main must both execute")
	assert.Equal(t, []string{"go", "build", "./..."}, rec.calls[0])
	assert.Equal(t, []string{"go", "test", "./..."}, rec.calls[1])
}

// TestDispatcher_MacroRun verifies macro mode resolves the router macro runner
// rather than falling through to a generic not-implemented branch.
func TestDispatcher_MacroRun(t *testing.T) {
	t.Parallel()

	runner := &stubMacroRunner{}

	d := cliwrapperadapter.NewWrapperDispatcherWithResolvers(
		cliwrapperadapter.WrapperConfig{
			Macros: []cliwrapperadapter.WrapperMacroConfig{{Name: "ci"}},
		},
		nil,
		cliwrapperadapter.WrapperResolvers{
			Core:        stubCoreResolver(),
			MacroRunner: func() (ports.CLIWrapperMacroRunner, error) { return runner, nil },
		},
	)
	err := d.Dispatch(context.Background(), []string{"ci", "--dry-run"})

	require.NoError(t, err)
	assert.Equal(t, "ci", runner.name)
}

// TestDispatcher_FormatHeaders verifies format-header args are parsed and
// delegated through the formatter port.
func TestDispatcher_FormatHeaders(t *testing.T) {
	t.Parallel()

	formatter := &stubFormatter{}

	d := cliwrapperadapter.NewWrapperDispatcherWithResolvers(
		cliwrapperadapter.WrapperConfig{},
		nil,
		cliwrapperadapter.WrapperResolvers{
			Core:      stubCoreResolver(),
			Formatter: func() (ports.CLIWrapperFormatter, error) { return formatter, nil },
		},
	)
	err := d.Dispatch(
		context.Background(),
		[]string{"go", "fmt", "headers", "--dry-run", "--only", "go", "python"},
	)

	require.NoError(t, err)
	assert.True(t, formatter.dryRun)
	assert.Equal(t, []string{"go", "python"}, formatter.only)
}

// TestDispatcher_PolicycheckSeparation verifies that a raw policycheck call is
// treated as passthrough, not as a wrapper-managed command.
func TestDispatcher_PolicycheckSeparation(t *testing.T) {
	t.Parallel()

	rec := &execRecorder{}

	d := cliwrapperadapter.NewWrapperDispatcherWithResolvers(
		cliwrapperadapter.WrapperConfig{},
		rec.exec,
		cliwrapperadapter.WrapperResolvers{Core: stubCoreResolver()},
	)
	err := d.Dispatch(context.Background(), []string{"policycheck", "--policy-list"})

	require.NoError(t, err)
	require.Len(t, rec.calls, 1)
	assert.Equal(t, []string{"policycheck", "--policy-list"}, rec.calls[0])
}
