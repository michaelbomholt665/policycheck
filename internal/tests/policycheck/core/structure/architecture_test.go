// internal/tests/policycheck/core/structure/architecture_test.go
package structure_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/core/structure"
)

func TestValidateArchitectureEntry(t *testing.T) {
	allowed := map[string]struct{}{
		"pkg1": {},
		"pkg2": {},
	}
	ignored := map[string]struct{}{
		"README.md": {},
		".git":      {},
	}

	tests := []struct {
		name     string
		entry    string
		isDir    bool
		expected bool // true if violation
	}{
		{"allowed dir", "pkg1", true, false},
		{"disallowed dir", "badpkg", true, true},
		{"ignored file", "README.md", false, false},
		{"disallowed file (but only dirs are checked)", "extra.go", false, false},
		{"ignored dir", ".git", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viol, ok := structure.ValidateArchitectureEntry("root", tt.entry, tt.isDir, allowed, ignored, []string{"pkg1", "pkg2"})
			assert.Equal(t, tt.expected, ok)
			if tt.expected {
				assert.NotEmpty(t, viol.Message)
				assert.Equal(t, "root/badpkg", viol.File)
			}
		})
	}
}

func TestCheckArchitectureIntegration(t *testing.T) {
	tmp := t.TempDir()

	// Setup directory structure
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "internal/allowed"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "internal/disallowed"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "internal/README.md"), []byte("text"), 0o644))

	cfg := config.PolicyConfig{
		Architecture: config.PolicyArchitectureConfig{
			Enforce: true,
			Roots: []config.PolicyArchitectureRoot{
				{
					Path:            "internal",
					AllowedChildren: []string{"allowed"},
					IgnoreChildren:  []string{"README.md"},
				},
			},
		},
	}

	violations := structure.CheckArchitecture(context.Background(), tmp, cfg)

	assert.Len(t, violations, 1)
	assert.Contains(t, violations[0].File, "internal/disallowed")
	assert.Contains(t, violations[0].Message, "allowed children: allowed")
}

func TestCheckArchitecture_NotEnforced(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "internal/disallowed"), 0o755))

	cfg := config.PolicyConfig{
		Architecture: config.PolicyArchitectureConfig{
			Enforce: false,
			Roots: []config.PolicyArchitectureRoot{
				{
					Path:            "internal",
					AllowedChildren: []string{"allowed"},
				},
			},
		},
	}

	violations := structure.CheckArchitecture(context.Background(), tmp, cfg)
	assert.Empty(t, violations)
}
