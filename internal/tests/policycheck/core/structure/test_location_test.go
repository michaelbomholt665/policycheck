// internal/tests/policycheck/core/structure/test_location_test.go
package structure_test

import (
	"context"
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

func setupTest(t *testing.T) {
	router.RouterResetForTest()
	exts := []router.Extension{
		walk.ExtensionInstance(),
	}
	_, err := router.RouterLoadExtensions(nil, exts, context.Background())
	require.NoError(t, err)
}

func TestCheckTestLocation(t *testing.T) {
	setupTest(t)
	defer router.RouterResetForTest()

	tmp := t.TempDir()

	// 1. Valid test location
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "internal/tests"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "internal/tests/valid_test.go"), []byte("package tests"), 0o644))

	// 2. Invalid test location (in production code)
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "internal/app"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "internal/app/bad_test.go"), []byte("package app"), 0o644))

	// 3. Normal production file (should be ignored)
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "internal/app/app.go"), []byte("package app"), 0o644))

	cfg := config.PolicyConfig{
		Paths: config.PolicyPathsConfig{
			TestScanRoots:       []string{"internal"},
			AllowedTestPrefixes: []string{"internal/tests/"},
		},
	}

	violations := structure.CheckTestLocation(context.Background(), tmp, cfg)

	assert.Len(t, violations, 1)
	assert.Equal(t, "structure.test_location", violations[0].RuleID)
	assert.Contains(t, violations[0].File, "bad_test.go")
	assert.Equal(t, "test files must be located in internal/tests/ to adhere to testing standards", violations[0].Message)
}
