package integration_test

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"policycheck/internal/ports"
	"policycheck/internal/router"
	"policycheck/internal/router/ext"
)

const bootTimeout = 10 * time.Second

func TestWrapperRouterBoot_RegistersRealMacroAndFormatterProviders(t *testing.T) {
	router.RouterResetForTest()
	t.Cleanup(router.RouterResetForTest)

	ctx, cancel := context.WithTimeout(context.Background(), bootTimeout)
	defer cancel()

	_, err := ext.RouterBootExtensions(ctx)
	require.NoError(t, err)

	macroProvider, err := router.RouterResolveProvider(router.PortCLIWrapperMacroRunner)
	require.NoError(t, err)
	_, ok := macroProvider.(ports.CLIWrapperMacroRunner)
	require.True(t, ok)
	assert.NotContains(t, reflect.TypeOf(macroProvider).String(), "Placeholder")

	formatProvider, err := router.RouterResolveProvider(router.PortCLIWrapperFormatter)
	require.NoError(t, err)
	_, ok = formatProvider.(ports.CLIWrapperFormatter)
	require.True(t, ok)
	assert.NotContains(t, reflect.TypeOf(formatProvider).String(), "Placeholder")
}

func TestWrapperAdapters_DoNotImportOtherAdapters(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "..", "adapters", "cliwrapper")
	entries, err := os.ReadDir(root)
	require.NoError(t, err)

	fileSet := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}

		path := filepath.Join(root, entry.Name())
		file, err := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
		require.NoErrorf(t, err, "parse imports for %s", path)

		for _, spec := range file.Imports {
			importPath := strings.Trim(spec.Path.Value, `"`)
			assert.Falsef(
				t,
				strings.HasPrefix(importPath, "policycheck/internal/adapters/"),
				"wrapper adapter %s must not import another adapter: %s",
				entry.Name(),
				importPath,
			)
		}
	}
}
