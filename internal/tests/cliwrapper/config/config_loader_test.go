// internal/tests/cliwrapper/config/config_loader_test.go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"policycheck/internal/cliwrapper"
)

// --- T1: Config schema tests ---

// TestWrapperConfig_ValidateWrapperConfig_EmptyIsValid confirms that a zero-value
// WrapperConfig passes validation (all fields are optional at this layer).
func TestWrapperConfig_ValidateWrapperConfig_EmptyIsValid(t *testing.T) {
	t.Parallel()

	err := cliwrapper.ValidateWrapperConfig(cliwrapper.WrapperConfig{})
	assert.NoError(t, err)
}

// TestWrapperConfig_Security_BlockThreshold_Recognised exercises each known
// severity label to confirm schema acceptance.
func TestWrapperConfig_Security_BlockThreshold_Recognised(t *testing.T) {
	t.Parallel()

	labels := []string{"low", "medium", "high", "critical"}

	for _, label := range labels {
		label := label
		t.Run(label, func(t *testing.T) {
			t.Parallel()

			cfg := cliwrapper.WrapperConfig{
				Security: cliwrapper.WrapperSecurityConfig{BlockThreshold: label},
			}
			assert.NoError(t, cliwrapper.ValidateWrapperConfig(cfg))
		})
	}
}

// TestWrapperConfig_Security_BlockThreshold_Unknown fails when an unrecognised
// severity label is supplied.
func TestWrapperConfig_Security_BlockThreshold_Unknown(t *testing.T) {
	t.Parallel()

	cfg := cliwrapper.WrapperConfig{
		Security: cliwrapper.WrapperSecurityConfig{BlockThreshold: "extreme"},
	}
	err := cliwrapper.ValidateWrapperConfig(cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "extreme")
}

// TestWrapperConfig_Macro_EmptyNameFails verifies that a macro with no name is rejected.
func TestWrapperConfig_Macro_EmptyNameFails(t *testing.T) {
	t.Parallel()

	cfg := cliwrapper.WrapperConfig{
		Macros: []cliwrapper.WrapperMacroConfig{
			{Name: "", Steps: []string{"go build ./..."}},
		},
	}
	err := cliwrapper.ValidateWrapperConfig(cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "name must not be empty")
}

// TestWrapperConfig_Macro_EmptyStepsFails verifies that a named macro with no
// steps is rejected.
func TestWrapperConfig_Macro_EmptyStepsFails(t *testing.T) {
	t.Parallel()

	cfg := cliwrapper.WrapperConfig{
		Macros: []cliwrapper.WrapperMacroConfig{
			{Name: "build", Steps: nil},
		},
	}
	err := cliwrapper.ValidateWrapperConfig(cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "steps must not be empty")
}

// TestValidateConfigStrictnessOrder_RepoMoreStrict accepts a stricter repo threshold.
func TestValidateConfigStrictnessOrder_RepoMoreStrict(t *testing.T) {
	t.Parallel()

	global := cliwrapper.WrapperConfig{Security: cliwrapper.WrapperSecurityConfig{BlockThreshold: "medium"}}
	repo := cliwrapper.WrapperConfig{Security: cliwrapper.WrapperSecurityConfig{BlockThreshold: "high"}}

	assert.NoError(t, cliwrapper.ValidateConfigStrictnessOrder(global, repo))
}

// TestValidateConfigStrictnessOrder_RepoRelaxes fails when repo is less strict.
func TestValidateConfigStrictnessOrder_RepoRelaxes(t *testing.T) {
	t.Parallel()

	global := cliwrapper.WrapperConfig{Security: cliwrapper.WrapperSecurityConfig{BlockThreshold: "high"}}
	repo := cliwrapper.WrapperConfig{Security: cliwrapper.WrapperSecurityConfig{BlockThreshold: "low"}}

	err := cliwrapper.ValidateConfigStrictnessOrder(global, repo)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not relax")
}

// TestValidateConfigStrictnessOrder_SameLevel accepts the same threshold in both configs.
func TestValidateConfigStrictnessOrder_SameLevel(t *testing.T) {
	t.Parallel()

	global := cliwrapper.WrapperConfig{Security: cliwrapper.WrapperSecurityConfig{BlockThreshold: "high"}}
	repo := cliwrapper.WrapperConfig{Security: cliwrapper.WrapperSecurityConfig{BlockThreshold: "high"}}

	assert.NoError(t, cliwrapper.ValidateConfigStrictnessOrder(global, repo))
}

// --- T2: Config loader tests ---

// TestWrapperConfigLoader_GlobalOnly_Loads verifies that a loader with only a
// global config and a start dir that lacks a repo config returns the global values.
func TestWrapperConfigLoader_GlobalOnly_Loads(t *testing.T) {
	t.Parallel()

	globalFile := writeTomlFile(t, `
[security]
block_threshold = "high"
`)
	startDir := t.TempDir() // no wrapper-gate.toml here

	loader := cliwrapper.WrapperConfigLoader{
		GlobalConfigPath: globalFile,
		StartDir:         startDir,
	}

	result, err := loader.Load()
	require.NoError(t, err)
	assert.Equal(t, "high", result.Merged.Security.BlockThreshold)
	assert.Equal(t, globalFile, result.GlobalPath)
	assert.Empty(t, result.RepoPath)
}

// TestWrapperConfigLoader_RepoOverridesGlobal verifies that a repo config with
// a stricter threshold overrides the global value.
func TestWrapperConfigLoader_RepoOverridesGlobal(t *testing.T) {
	t.Parallel()

	globalFile := writeTomlFile(t, `
[security]
block_threshold = "medium"
`)
	startDir := writeDirWithToml(t, `
[security]
block_threshold = "critical"
`)

	loader := cliwrapper.WrapperConfigLoader{
		GlobalConfigPath: globalFile,
		StartDir:         startDir,
	}

	result, err := loader.Load()
	require.NoError(t, err)
	assert.Equal(t, "critical", result.Merged.Security.BlockThreshold)
	assert.NotEmpty(t, result.RepoPath)
}

// TestWrapperConfigLoader_RepoRelaxesGlobal_Fails confirms that a repo config
// whose threshold is less strict than global is rejected.
func TestWrapperConfigLoader_RepoRelaxesGlobal_Fails(t *testing.T) {
	t.Parallel()

	globalFile := writeTomlFile(t, `
[security]
block_threshold = "high"
`)
	startDir := writeDirWithToml(t, `
[security]
block_threshold = "low"
`)

	loader := cliwrapper.WrapperConfigLoader{
		GlobalConfigPath: globalFile,
		StartDir:         startDir,
	}

	_, err := loader.Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not relax")
}

// TestWrapperConfigLoader_MissingRepo_FallsBackToGlobal confirms that a missing
// repo config is not an error and global values are preserved.
func TestWrapperConfigLoader_MissingRepo_FallsBackToGlobal(t *testing.T) {
	t.Parallel()

	globalFile := writeTomlFile(t, `
[security]
block_threshold = "medium"
`)
	startDir := t.TempDir()

	loader := cliwrapper.WrapperConfigLoader{
		GlobalConfigPath: globalFile,
		StartDir:         startDir,
	}

	result, err := loader.Load()
	require.NoError(t, err)
	assert.Equal(t, "medium", result.Merged.Security.BlockThreshold)
	assert.Empty(t, result.RepoPath)
}

// TestWrapperConfigLoader_NoGlobalNoRepo_EmptyConfig proves that a loader
// with no global path and no repo file returns an empty merged config without
// error.
func TestWrapperConfigLoader_NoGlobalNoRepo_EmptyConfig(t *testing.T) {
	t.Parallel()

	loader := cliwrapper.WrapperConfigLoader{
		StartDir: t.TempDir(),
	}

	result, err := loader.Load()
	require.NoError(t, err)
	assert.Empty(t, result.GlobalPath)
	assert.Empty(t, result.RepoPath)
	assert.Equal(t, cliwrapper.WrapperConfig{}, result.Merged)
}

// --- helpers ---

// writeTomlFile creates a temporary TOML file with the given content and
// returns its path. The file is cleaned up after the test.
func writeTomlFile(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "wrapper-gate.toml")

	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err, "writeTomlFile: write failed")

	return path
}

// writeDirWithToml creates a temporary directory containing a wrapper-gate.toml
// file with the given content. Returns the directory path.
func writeDirWithToml(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "wrapper-gate.toml")

	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err, "writeDirWithToml: write failed")

	return dir
}
