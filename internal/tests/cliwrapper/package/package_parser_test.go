// internal/tests/cliwrapper/package/package_parser_test.go
package package_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"policycheck/internal/cliwrapper"
)

// TestParseInstallRequest_SupportedManagers exercises the happy path for each
// supported package manager.
func TestParseInstallRequest_SupportedManagers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        []string
		wantManager string
		wantPkgs    []string
	}{
		{
			name:        "npm install single package",
			args:        []string{"npm", "install", "lodash"},
			wantManager: "npm",
			wantPkgs:    []string{"lodash"},
		},
		{
			name:        "npm i alias",
			args:        []string{"npm", "i", "express", "axios"},
			wantManager: "npm",
			wantPkgs:    []string{"express", "axios"},
		},
		{
			name:        "pip install with version pin",
			args:        []string{"pip", "install", "requests==2.31.0"},
			wantManager: "pip",
			wantPkgs:    []string{"requests==2.31.0"},
		},
		{
			name:        "go get module",
			args:        []string{"go", "get", "golang.org/x/sync"},
			wantManager: "go",
			wantPkgs:    []string{"golang.org/x/sync"},
		},
		{
			name:        "uv add package",
			args:        []string{"uv", "add", "httpx"},
			wantManager: "uv",
			wantPkgs:    []string{"httpx"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req, err := cliwrapper.ParseInstallRequest(tc.args)
			require.NoError(t, err)

			assert.Equal(t, tc.wantManager, req.Manager)
			assert.Equal(t, tc.args, req.RawArgs)
			assert.Equal(t, tc.wantPkgs, req.Packages)
			assert.NotEmpty(t, req.LockfileHint, "lockfile hint must be set for known managers")
		})
	}
}

// TestParseInstallRequest_UnsupportedManager verifies that unknown managers
// fail loudly with a typed error.
func TestParseInstallRequest_UnsupportedManager(t *testing.T) {
	t.Parallel()

	_, err := cliwrapper.ParseInstallRequest([]string{"cargo", "add", "serde"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, cliwrapper.ErrUnsupportedManager),
		"expected ErrUnsupportedManager, got: %v", err)
}

// TestParseInstallRequest_EmptyArgs verifies that an empty arg slice fails.
func TestParseInstallRequest_EmptyArgs(t *testing.T) {
	t.Parallel()

	_, err := cliwrapper.ParseInstallRequest(nil)
	require.Error(t, err)
}

// TestParseInstallRequest_MissingSubcommand verifies that a manager without
// an install subcommand fails with ErrUnsupportedManager.
func TestParseInstallRequest_MissingSubcommand(t *testing.T) {
	t.Parallel()

	// "npm build" is not an install subcommand.
	_, err := cliwrapper.ParseInstallRequest([]string{"npm", "build", "myapp"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, cliwrapper.ErrUnsupportedManager))
}
