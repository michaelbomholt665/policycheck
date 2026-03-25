// internal/tests/policycheck/core/hygiene/integration_test.go
package hygiene_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"policycheck/internal/adapters/walk"
	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/core/hygiene"
	"policycheck/internal/router"
)

func setupIntegrationTest(t *testing.T) {
	router.RouterResetForTest()
	exts := []router.Extension{
		walk.ExtensionInstance(),
	}
	_, err := router.RouterLoadExtensions(nil, exts, context.Background())
	require.NoError(t, err)
}

func TestHygieneIntegration(t *testing.T) {
	setupIntegrationTest(t)
	defer router.RouterResetForTest()

	tmp := t.TempDir()

	// Create a Go file with naming and doc violations
	content := `package test

// WrongName does something
func CorrectName() {}

func bad() {}

// ExportedWithoutDoc is a dummy variable for testing.
var ExportedWithoutDoc = 1

type Bad int

func Exported() {}
`
	err := os.WriteFile(filepath.Join(tmp, "bad.go"), []byte(content), 0o644)
	require.NoError(t, err)

	cfg := config.PolicyConfig{
		Hygiene: config.PolicyHygieneConfig{
			ScanRoots:     []string{"."},
			MinNameTokens: 2,
		},
	}

	t.Run("CheckSymbolNames", func(t *testing.T) {
		violations := hygiene.CheckSymbolNames(context.Background(), tmp, cfg)
		// Expected: Bad (1 token), Exported (1 token)
		// ExportedWithoutDoc (3 tokens) -> pass
		// CorrectName (2 tokens) -> pass
		// bad (unexported) -> skip

		names := []string{}
		for _, v := range violations {
			names = append(names, v.Message)
		}

		assert.Len(t, violations, 2)
		assert.Contains(t, names[0], "Bad")
		assert.Contains(t, names[1], "Exported")
	})

	t.Run("CheckDocStyle", func(t *testing.T) {
		violations := hygiene.CheckDocStyle(context.Background(), tmp, cfg)
		// Expected:
		// CorrectName: comment starts with WrongName -> violation
		// Bad: no comment -> violation
		// Exported: no comment -> violation
		// ExportedWithoutDoc: has comment starting with name -> pass

		assert.Len(t, violations, 3)
		assert.Contains(t, violations[0].Message, "CorrectName")
		assert.Contains(t, violations[1].Message, "Bad")
		assert.Contains(t, violations[2].Message, "Exported")
	})

	t.Run("ExcludePrefixes", func(t *testing.T) {
		os.MkdirAll(filepath.Join(tmp, "excluded"), 0o755)
		content := `package excluded
func Bad() {}`
		err := os.WriteFile(filepath.Join(tmp, "excluded", "bad.go"), []byte(content), 0o644)
		require.NoError(t, err)

		cfgExcluded := cfg
		cfgExcluded.Hygiene.ExcludePrefixes = []string{"excluded"}

		violations := hygiene.CheckSymbolNames(context.Background(), tmp, cfgExcluded)
		// Should still only have 2 violations from the root bad.go, skipping excluded/bad.go
		assert.Len(t, violations, 2)
	})

	t.Run("ScanRoots", func(t *testing.T) {
		os.MkdirAll(filepath.Join(tmp, "other"), 0o755)
		content := `package other
func Bad() {}`
		err := os.WriteFile(filepath.Join(tmp, "other", "bad.go"), []byte(content), 0o644)
		require.NoError(t, err)

		cfgRoots := cfg
		cfgRoots.Hygiene.ScanRoots = []string{"other"}

		violations := hygiene.CheckSymbolNames(context.Background(), tmp, cfgRoots)
		// Should only find the 1 violation in "other/bad.go"
		assert.Len(t, violations, 1)
		assert.Contains(t, violations[0].File, "other")
	})
}
