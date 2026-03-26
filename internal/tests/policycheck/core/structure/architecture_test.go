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
			rule := structure.ArchitectureEntryRule{
				RulePath:    "root",
				Allowed:     allowed,
				Ignored:     ignored,
				AllowedList: []string{"pkg1", "pkg2"},
			}
			viol, ok := structure.ValidateArchitectureEntry(rule, tt.entry, tt.isDir)
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

func TestCheckArchitecture_ConcernPaths(t *testing.T) {
	tmp := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "internal/db"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "docs/database"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "internal/db/schema.go"), []byte("package db"), 0o644))

	cfg := config.PolicyConfig{
		Architecture: config.PolicyArchitectureConfig{
			Enforce: true,
			Concerns: []config.PolicyArchitectureTopic{
				{
					Name:          "database",
					Tags:          []string{"database"},
					Roots:         []string{"internal/db"},
					ConfigPaths:   []string{"internal/config"},
					SchemaPaths:   []string{"internal/db/schema.go"},
					ContractPaths: []string{"docs/database"},
					APIPaths:      []string{"internal/db"},
				},
			},
		},
	}

	violations := structure.CheckArchitecture(context.Background(), tmp, cfg)

	require.Len(t, violations, 1)
	assert.Equal(t, "structure.architecture", violations[0].RuleID)
	assert.Contains(t, violations[0].Message, `references missing config_paths entry "internal/config"`)
}
