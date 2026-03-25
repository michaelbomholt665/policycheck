// internal/tests/policycheck/core/structure/package_rules_test.go
package structure_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"policycheck/internal/adapters/walk"
	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/core/structure"
	"policycheck/internal/router"
)

func TestParseDocGoConcerns(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		expectedHas   bool
		expectedCount int
	}{
		{
			"valid concerns",
			`// Package foo
//
// Package Concerns:
// - Concern 1
// - Concern 2
package foo`,
			true,
			2,
		},
		{
			"no concerns",
			`// Package foo
package foo`,
			false,
			0,
		},
		{
			"empty concerns",
			`// Package foo
//
// Package Concerns:
package foo`,
			true,
			0,
		},
		{
			"concerns with other bullets",
			`// Package foo
//
// Package Concerns:
// - Concern 1
//
// Other:
// - Not a concern
package foo`,
			true,
			1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			has, count := structure.ParseDocGoConcerns(tt.content)
			assert.Equal(t, tt.expectedHas, has)
			assert.Equal(t, tt.expectedCount, count)
		})
	}
}

func TestValidatePackageStats(t *testing.T) {
	cfg := config.PolicyPackageRulesConfig{
		MaxProductionFiles: 10,
		MinConcerns:        1,
		MaxConcerns:        2,
	}

	tests := []struct {
		name     string
		stats    map[string]*structure.PackageStats
		expected int // number of violations
	}{
		{
			"valid package",
			map[string]*structure.PackageStats{
				"pkg": {ProductionGo: 5, HasDoc: true, DocPrefixOK: true, HasConcerns: true, ConcernCount: 1},
			},
			0,
		},
		{
			"too many files",
			map[string]*structure.PackageStats{
				"pkg": {ProductionGo: 11, HasDoc: true, DocPrefixOK: true, HasConcerns: true, ConcernCount: 1},
			},
			1,
		},
		{
			"missing doc.go",
			map[string]*structure.PackageStats{
				"pkg": {ProductionGo: 5, HasDoc: false},
			},
			1,
		},
		{
			"doc.go bad prefix",
			map[string]*structure.PackageStats{
				"pkg": {ProductionGo: 5, HasDoc: true, DocPrefixOK: false, HasConcerns: true, ConcernCount: 1, DocPath: "pkg/doc.go"},
			},
			1,
		},
		{
			"too many concerns",
			map[string]*structure.PackageStats{
				"pkg": {ProductionGo: 5, HasDoc: true, DocPrefixOK: true, HasConcerns: true, ConcernCount: 3, DocPath: "pkg/doc.go"},
			},
			1,
		},
		{
			"too few concerns",
			map[string]*structure.PackageStats{
				"pkg": {ProductionGo: 5, HasDoc: true, DocPrefixOK: true, HasConcerns: true, ConcernCount: 0, DocPath: "pkg/doc.go"},
			},
			1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viols := structure.ValidatePackageStats(tt.stats, cfg)
			assert.Len(t, viols, tt.expected)
		})
	}
}

func TestCheckPackageRulesIntegration(t *testing.T) {
	router.RouterResetForTest()
	exts := []router.Extension{
		walk.ExtensionInstance(),
	}
	_, err := router.RouterLoadExtensions(nil, exts, context.Background())
	require.NoError(t, err)
	defer router.RouterResetForTest()

	tmp := t.TempDir()

	// 1. Valid package
	pkg1 := filepath.Join(tmp, "internal/pkg1")
	require.NoError(t, os.MkdirAll(pkg1, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pkg1, "doc.go"), []byte("// Package pkg1 Is a test\n//\n// Package Concerns:\n// - Concern 1\npackage pkg1"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(pkg1, "file1.go"), []byte("package pkg1"), 0o644))

	// 2. Invalid package (too many files, missing doc.go)
	pkg2 := filepath.Join(tmp, "internal/pkg2")
	require.NoError(t, os.MkdirAll(pkg2, 0o755))
	for i := 1; i <= 3; i++ {
		require.NoError(t, os.WriteFile(filepath.Join(pkg2, fmt.Sprintf("file%d.go", i)), []byte("package pkg2"), 0o644))
	}

	cfg := config.PolicyConfig{
		PackageRules: config.PolicyPackageRulesConfig{
			ScanRoots:          []string{"internal"},
			MaxProductionFiles: 2,
			MinConcerns:        1,
			MaxConcerns:        2,
		},
	}

	violations := structure.CheckPackageRules(context.Background(), tmp, cfg)

	// Expected:
	// pkg1: 1 production file -> WARN
	// pkg2: 3 production files > 2 -> error
	// pkg2: missing doc.go -> error
	assert.Len(t, violations, 3)
}

func TestCheckPackageRulesExcludePrefixes(t *testing.T) {
	router.RouterResetForTest()
	exts := []router.Extension{
		walk.ExtensionInstance(),
	}
	_, err := router.RouterLoadExtensions(nil, exts, context.Background())
	require.NoError(t, err)
	defer router.RouterResetForTest()

	tmp := t.TempDir()
	pkg := filepath.Join(tmp, "internal", "adapters", "config")
	require.NoError(t, os.MkdirAll(pkg, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pkg, "doc.go"), []byte("// Package config Is a test\n//\n// Package Concerns:\n// - Concern 1\npackage config"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(pkg, "extension.go"), []byte("package config"), 0o644))

	cfg := config.PolicyConfig{
		PackageRules: config.PolicyPackageRulesConfig{
			ScanRoots:          []string{"internal"},
			ExcludePrefixes:    []string{"internal/adapters/config"},
			MaxProductionFiles: 10,
			MinConcerns:        1,
			MaxConcerns:        2,
		},
	}

	violations := structure.CheckPackageRules(context.Background(), tmp, cfg)
	assert.Empty(t, violations)
}
