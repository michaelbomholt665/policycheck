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

func TestWrapperConfig_ValidateWrapperConfig_EmptyIsValid(t *testing.T) {
	t.Parallel()

	err := cliwrapper.ValidateWrapperConfig(cliwrapper.WrapperConfig{})
	assert.NoError(t, err)
}

func TestWrapperConfig_Security_BlockOn_Recognised(t *testing.T) {
	t.Parallel()

	labels := []string{"info", "low", "moderate", "medium", "high", "critical"}

	for _, label := range labels {
		label := label
		t.Run(label, func(t *testing.T) {
			t.Parallel()

			cfg := cliwrapper.WrapperConfig{
				Security: cliwrapper.WrapperSecurityConfig{BlockOn: []string{label}},
			}
			assert.NoError(t, cliwrapper.ValidateWrapperConfig(cfg))
		})
	}
}

func TestWrapperConfig_Security_BlockOn_Unknown(t *testing.T) {
	t.Parallel()

	cfg := cliwrapper.WrapperConfig{
		Security: cliwrapper.WrapperSecurityConfig{BlockOn: []string{"extreme"}},
	}
	err := cliwrapper.ValidateWrapperConfig(cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "extreme")
}

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

func TestValidateConfigStrictnessOrder_RepoMoreStrict(t *testing.T) {
	t.Parallel()

	global := cliwrapper.WrapperConfig{Security: cliwrapper.WrapperSecurityConfig{BlockOn: []string{"CRITICAL", "HIGH"}}}
	repo := cliwrapper.WrapperConfig{Security: cliwrapper.WrapperSecurityConfig{BlockOn: []string{"CRITICAL", "HIGH", "MODERATE"}}}

	assert.NoError(t, cliwrapper.ValidateConfigStrictnessOrder(global, repo))
}

func TestValidateConfigStrictnessOrder_RepoRelaxes(t *testing.T) {
	t.Parallel()

	global := cliwrapper.WrapperConfig{Security: cliwrapper.WrapperSecurityConfig{BlockOn: []string{"CRITICAL", "HIGH"}}}
	repo := cliwrapper.WrapperConfig{Security: cliwrapper.WrapperSecurityConfig{BlockOn: []string{"CRITICAL"}}}

	err := cliwrapper.ValidateConfigStrictnessOrder(global, repo)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not relax")
}

func TestValidateConfigStrictnessOrder_SameLevel(t *testing.T) {
	t.Parallel()

	global := cliwrapper.WrapperConfig{Security: cliwrapper.WrapperSecurityConfig{BlockOn: []string{"CRITICAL", "HIGH"}}}
	repo := cliwrapper.WrapperConfig{Security: cliwrapper.WrapperSecurityConfig{BlockOn: []string{"CRITICAL", "HIGH"}}}

	assert.NoError(t, cliwrapper.ValidateConfigStrictnessOrder(global, repo))
}

func TestWrapperConfigLoader_GlobalOnly_Loads(t *testing.T) {
	t.Parallel()

	globalFile := writeTomlFile(t, `
[security]
block_on = ["CRITICAL", "HIGH"]
`)
	startDir := t.TempDir()

	loader := cliwrapper.WrapperConfigLoader{
		GlobalConfigPath: globalFile,
		StartDir:         startDir,
	}

	result, err := loader.Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"CRITICAL", "HIGH"}, result.Merged.Security.BlockOn)
	assert.Equal(t, []string{"MODERATE"}, result.Merged.Security.WarnOn)
	assert.Equal(t, []string{"LOW", "INFO"}, result.Merged.Security.AllowOn)
	assert.Equal(t, globalFile, result.GlobalPath)
	assert.Empty(t, result.RepoPath)
}

func TestWrapperConfigLoader_UsesPolicyGateTomlDiscovery(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	nestedDir := filepath.Join(rootDir, "a", "b", "c")
	require.NoError(t, os.MkdirAll(nestedDir, 0o755))
	writeNamedTomlFile(t, rootDir, cliwrapper.RepoConfigFilename, `
[security]
block_on = ["CRITICAL", "HIGH", "MODERATE"]
`)
	writeNamedTomlFile(t, rootDir, "wrapper-gate.toml", `
[security]
block_on = ["LOW"]
`)

	loader := cliwrapper.WrapperConfigLoader{
		StartDir: nestedDir,
	}

	result, err := loader.Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"CRITICAL", "HIGH", "MODERATE"}, result.Merged.Security.BlockOn)
	assert.NotEmpty(t, result.RepoPath)
	assert.Equal(t, filepath.Join(rootDir, cliwrapper.RepoConfigFilename), result.RepoPath)
}

func TestWrapperConfigLoader_RepoRelaxesGlobal_Fails(t *testing.T) {
	t.Parallel()

	globalFile := writeTomlFile(t, `
[security]
block_on = ["CRITICAL", "HIGH"]
`)
	startDir := writeDirWithToml(t, `
[security]
block_on = ["CRITICAL"]
`)

	loader := cliwrapper.WrapperConfigLoader{
		GlobalConfigPath: globalFile,
		StartDir:         startDir,
	}

	_, err := loader.Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not relax")
}

func TestWrapperConfigLoader_MergesMacrosAndToolingByKey(t *testing.T) {
	t.Parallel()

	globalFile := writeTomlFile(t, `
[tooling.gates.go-test]
gate = "gofumpt -l ./..."
run = "go test ./..."

[macros.ci]
steps = ["go test ./..."]
on_failure = "stop"
`)
	startDir := writeDirWithToml(t, `
[tooling.gates.go-test]
gate = "gofumpt -w ."
run = "go test ./..."

[tooling.gates.py-test]
gate = "ruff check ."
run = "python -m pytest"

[macros.ci]
steps = ["go test ./...", "python -m pytest"]
on_failure = "stop"

[macros.release]
steps = ["git tag v1.0.0"]
on_failure = "continue"
`)

	loader := cliwrapper.WrapperConfigLoader{
		GlobalConfigPath: globalFile,
		StartDir:         startDir,
	}

	result, err := loader.Load()
	require.NoError(t, err)

	require.Len(t, result.Merged.Tooling.Gates, 2)
	assert.Equal(t, "go-test", result.Merged.Tooling.Gates[0].Name)
	assert.Equal(t, "gofumpt -w .", result.Merged.Tooling.Gates[0].Gate)
	assert.Equal(t, "py-test", result.Merged.Tooling.Gates[1].Name)

	require.Len(t, result.Merged.Macros, 2)
	assert.Equal(t, "ci", result.Merged.Macros[0].Name)
	assert.Equal(t, []string{"go test ./...", "python -m pytest"}, result.Merged.Macros[0].Steps)
	assert.Equal(t, "release", result.Merged.Macros[1].Name)
}

func TestWrapperConfigLoader_MissingRepo_FallsBackToDefaults(t *testing.T) {
	t.Parallel()

	loader := cliwrapper.WrapperConfigLoader{
		StartDir: t.TempDir(),
	}

	result, err := loader.Load()
	require.NoError(t, err)
	assert.Empty(t, result.GlobalPath)
	assert.Empty(t, result.RepoPath)
	assert.Equal(t, cliwrapper.DefaultWrapperConfig().Security.BlockOn, result.Merged.Security.BlockOn)
	assert.Equal(t, cliwrapper.DefaultWrapperConfig().Security.WarnOn, result.Merged.Security.WarnOn)
}

func writeTomlFile(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, cliwrapper.RepoConfigFilename)

	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err, "writeTomlFile: write failed")

	return path
}

func writeDirWithToml(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, cliwrapper.RepoConfigFilename)

	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err, "writeDirWithToml: write failed")

	return dir
}

func writeNamedTomlFile(t *testing.T, dir string, name string, content string) {
	t.Helper()

	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err, "writeNamedTomlFile: write failed")
}
