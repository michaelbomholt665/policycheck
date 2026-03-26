// internal/tests/policycheck/config/defaults_test.go
package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"policycheck/internal/policycheck/config"
)

func TestApplyPolicyConfigDefaults(t *testing.T) {
	cfg := config.PolicyConfig{}

	err := config.ApplyPolicyConfigDefaults(&cfg)
	require.NoError(t, err)

	// Go Version
	assert.Equal(t, []string{"1.24", "1.25"}, cfg.GoVersion.AllowedPrefixes)

	// Hygiene
	assert.Equal(t, []string{"internal", "cmd"}, cfg.Hygiene.ScanRoots)
	assert.Equal(t, []string{"cmd/policycheck"}, cfg.Hygiene.ExcludePrefixes)
	assert.Equal(t, 2, cfg.Hygiene.MinNameTokens)
	assert.Equal(t, 3, cfg.Hygiene.CrossBackendMinNameTokens)

	// Package Rules
	assert.Equal(t, []string{"cmd", "internal", "test"}, cfg.PackageRules.ScanRoots)
	assert.Empty(t, cfg.PackageRules.ExcludePrefixes)
	assert.Equal(t, 10, cfg.PackageRules.MaxProductionFiles)
	assert.Equal(t, 1, cfg.PackageRules.MinConcerns)
	assert.Equal(t, 2, cfg.PackageRules.MaxConcerns)

	// Function Quality (new fields + old fields recalibrated)
	assert.Equal(t, []string{"go", "python", "typescript"}, cfg.FunctionQuality.EnabledLanguages)
	assert.Equal(t, 12, cfg.FunctionQuality.MildCTXMin)
	assert.Equal(t, 14, cfg.FunctionQuality.ElevatedCTXMin)
	assert.Equal(t, 16, cfg.FunctionQuality.ImmediateRefactorCTXMin)
	assert.Equal(t, 18, cfg.FunctionQuality.ErrorCTXMin)
	assert.Equal(t, 10, cfg.FunctionQuality.ErrorCTXAndLOCCTX)

	// Secret Logging (new fields)
	assert.Equal(t, []string{"example", "sample", "placeholder", "dummy", "fake", "fixture", "redacted", "masked"}, cfg.SecretLogging.BenignHints)
	assert.Equal(t, []string{"<token>", "<password>", "<secret>", "<api-key>", "changeme", "change_me", "replace_me", "your_token_here"}, cfg.SecretLogging.PlaceholderStrings)

	// AI Compatibility
	assert.Equal(t, []string{"--ai", "--user"}, cfg.AICompatibility.RequiredFlags)

	// Scope Guard
	assert.Equal(t, config.ScopeGuardModeRestrict, cfg.ScopeGuard.Mode)
	assert.Equal(t, []string{
		"os.WriteFile",
		"os.Rename",
		"os.Remove",
		"os.RemoveAll",
		"os.Chmod",
		"os.Chown",
		"os.Mkdir",
		"os.MkdirAll",
	}, cfg.ScopeGuard.ForbiddenCalls)
	assert.Empty(t, cfg.ScopeGuard.AllowedPathPrefixes)

	// Router Imports
	assert.Equal(t, []string{"internal/policycheck", "internal/cliwrapper", "internal/ports"}, cfg.RouterImports.BusinessRoots)
	assert.Equal(t, []string{"internal/adapters"}, cfg.RouterImports.AdapterRoots)
	assert.Equal(t, []string{"internal/router"}, cfg.RouterImports.RouterCoreRoots)
	assert.Equal(t, []string{"internal/app", "internal/router/ext"}, cfg.RouterImports.RouterBootRoots)
	assert.Equal(t, []string{
		"policycheck/internal/ports",
		"policycheck/internal/router",
		"policycheck/internal/router/capabilities",
	}, cfg.RouterImports.AllowedBusinessImports)
	assert.Equal(t, []string{
		"policycheck/internal/adapters/",
		"policycheck/internal/router/ext/",
	}, cfg.RouterImports.ForbiddenBusinessImportPrefixes)
	assert.True(t, cfg.RouterImports.ForbiddenAdapterToAdapter)

	// Documentation
	assert.Equal(t, "loose", cfg.Documentation.Level)
	assert.Equal(t, []string{"internal", "cmd", "scripts"}, cfg.Documentation.ScanRoots)
	assert.Equal(t, "google", cfg.Documentation.GoStyle)
	assert.Equal(t, "numpy", cfg.Documentation.PythonStyle)
	assert.Equal(t, "tsdoc", cfg.Documentation.TypeScriptStyle)
	assert.Equal(t, []string{"scripts"}, cfg.Documentation.PythonShebangRoots)
}
