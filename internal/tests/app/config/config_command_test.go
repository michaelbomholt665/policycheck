package config_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"policycheck/internal/app"
	"policycheck/internal/cliwrapper"
)

func TestRunWithDependencies_ConfigCommand_RoutesToConfigHandler(t *testing.T) {
	t.Parallel()

	var configCalled bool
	var analysisCalled bool
	var wrapperCalled bool

	exitCode := app.RunWithDependencies(
		context.Background(),
		[]string{"config", "--global"},
		func(args []string) int {
			analysisCalled = true
			return 0
		},
		func(ctx context.Context, args []string) error {
			wrapperCalled = true
			return nil
		},
		func(args []string) error {
			configCalled = true
			assert.Equal(t, []string{"--global"}, args)
			return nil
		},
	)

	assert.Equal(t, 0, exitCode)
	assert.True(t, configCalled)
	assert.False(t, analysisCalled)
	assert.False(t, wrapperCalled)
}

func TestRunConfigCommand_Config_PrintsMergedConfig(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	globalPath, err := cliwrapper.DefaultGlobalConfigPath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(globalPath), 0o755))
	require.NoError(t, os.WriteFile(globalPath, []byte(`
[security]
block_on = ["CRITICAL", "HIGH"]
`), 0o600))

	repoDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, cliwrapper.RepoConfigFilename), []byte(`
[security]
block_on = ["CRITICAL", "HIGH", "MODERATE"]
`), 0o600))
	chdirForTest(t, filepath.Join(repoDir))

	output := captureStdout(t, func() {
		err = app.RunConfigCommand(nil)
	})

	require.NoError(t, err)
	assert.Contains(t, output, `scope = "merged"`)
	assert.Contains(t, output, "repo_path = "+filepath.Join(repoDir, cliwrapper.RepoConfigFilename))
	assert.Contains(t, output, `block_on = ['CRITICAL', 'HIGH', 'MODERATE']`)
}

func TestRunConfigCommand_Global_PrintsGlobalConfig(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	globalPath, err := cliwrapper.DefaultGlobalConfigPath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(globalPath), 0o755))
	require.NoError(t, os.WriteFile(globalPath, []byte(`
[security]
block_on = ["CRITICAL"]
`), 0o600))

	output := captureStdout(t, func() {
		err = app.RunConfigCommand([]string{"--global"})
	})

	require.NoError(t, err)
	assert.Contains(t, output, `scope = "global"`)
	assert.Contains(t, output, "global_path = "+globalPath)
	assert.Contains(t, output, `block_on = ['CRITICAL']`)
}

func TestRunConfigCommand_Init_DryRun_PrintsRepoScaffold(t *testing.T) {
	repoDir := t.TempDir()
	chdirForTest(t, repoDir)

	var err error
	output := captureStdout(t, func() {
		err = app.RunConfigCommand([]string{"init", "--dry-run"})
	})

	require.NoError(t, err)
	assert.Contains(t, output, "target_path = "+filepath.Join(repoDir, cliwrapper.RepoConfigFilename))
	assert.Contains(t, output, "[security]")
	assert.Contains(t, output, `block_on = ['CRITICAL', 'HIGH']`)
}

func TestRunConfigCommand_InitGlobal_DryRun_PrintsGlobalScaffold(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	globalPath, err := cliwrapper.DefaultGlobalConfigPath()
	require.NoError(t, err)

	output := captureStdout(t, func() {
		err = app.RunConfigCommand([]string{"init", "--global", "--dry-run"})
	})

	require.NoError(t, err)
	assert.Contains(t, output, "target_path = "+globalPath)
	assert.Contains(t, output, "[security]")
	assert.Contains(t, output, `osv_mode = 'cli'`)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = writer
	t.Cleanup(func() {
		os.Stdout = originalStdout
	})

	fn()

	require.NoError(t, writer.Close())
	data, err := io.ReadAll(reader)
	require.NoError(t, err)

	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

func chdirForTest(t *testing.T, dir string) {
	t.Helper()

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(originalDir))
	})
}
