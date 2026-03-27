package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"policycheck/internal/cliwrapper"
)

func TestWrapperRepoConfigFilename_RemainsPolicyGateToml(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "policy-gate.toml", cliwrapper.RepoConfigFilename)
}

func TestWrapperImplementation_DoesNotReferenceLegacyWrapperGateToml(t *testing.T) {
	t.Parallel()

	paths := []string{
		filepath.Join("..", "..", "..", "cliwrapper", "walker.go"),
		filepath.Join("..", "..", "..", "adapters", "cliwrapper", "format_headers.go"),
		filepath.Join("..", "..", "..", "cliwrapper", "config_loader.go"),
		filepath.Join("..", "..", "..", "app", "config_command.go"),
	}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		require.NoErrorf(t, err, "read %s", path)
		assert.NotContainsf(
			t,
			string(content),
			"wrapper-gate.toml",
			"wrapper implementation must not drift back to the legacy repo config filename in %s",
			path,
		)
	}
}
