package run_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunGo_ImportsOnlySharedEntrypointBoundaries(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "app", "run.go")
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
	require.NoError(t, err)

	allowedInternalImports := map[string]struct{}{
		"policycheck/internal/cliwrapper":      {},
		"policycheck/internal/policycheck/cli": {},
	}

	for _, spec := range file.Imports {
		importPath := strings.Trim(spec.Path.Value, `"`)
		if !strings.HasPrefix(importPath, "policycheck/internal/") {
			continue
		}

		_, allowed := allowedInternalImports[importPath]
		assert.Truef(
			t,
			allowed,
			"internal/app/run.go must stay on the shared entrypoint boundary, found import %s",
			importPath,
		)
	}
}
