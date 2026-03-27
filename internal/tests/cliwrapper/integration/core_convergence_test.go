package integration_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrapperAdapterMacroRunner_DoesNotShadowSharedCoreHelpers(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "..", "adapters", "cliwrapper", "macro_runner.go")
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
	require.NoError(t, err)

	duplicateHelpers := map[string]struct{}{
		"adapterMacroRunner":       {},
		"findMacroByName":          {},
		"normalizeMacroOnFailure":  {},
		"prepareAdapterMacroStep":  {},
		"interpolateMacroTemplate": {},
		"splitState":               {},
		"splitMacroCommandLine":    {},
	}

	found := make([]string, 0)
	for _, decl := range file.Decls {
		switch typed := decl.(type) {
		case *ast.FuncDecl:
			if _, ok := duplicateHelpers[typed.Name.Name]; ok {
				found = append(found, typed.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range typed.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, exists := duplicateHelpers[typeSpec.Name.Name]; exists {
					found = append(found, typeSpec.Name.Name)
				}
			}
		}
	}

	assert.Empty(t, found, "adapter macro runner must reuse internal/cliwrapper macro helpers")
}

func TestWrapperAdapterHeaderFormatter_DoesNotShadowSharedCoreHelpers(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "..", "adapters", "cliwrapper", "format_headers.go")
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
	require.NoError(t, err)

	duplicateHelpers := map[string]struct{}{
		"headerFormattingDeps":   {},
		"formatSingleFile":       {},
		"runHeaderFormatting":    {},
		"normalizeHeaderFilter":  {},
		"shouldSkipHeaderPath":   {},
		"applyHeaderForPath":     {},
		"applySlashHeader":       {},
		"applyPythonHeader":      {},
		"splitContentLines":      {},
		"joinContentLines":       {},
		"firstShebang":           {},
		"resolveAdapterRepoRoot": {},
	}

	found := make([]string, 0)
	for _, decl := range file.Decls {
		switch typed := decl.(type) {
		case *ast.FuncDecl:
			if _, ok := duplicateHelpers[typed.Name.Name]; ok {
				found = append(found, typed.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range typed.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, exists := duplicateHelpers[typeSpec.Name.Name]; exists {
					found = append(found, typeSpec.Name.Name)
				}
			}
		}
	}

	assert.Empty(t, found, "adapter header formatter must reuse internal/cliwrapper header helpers")
}

func TestWrapperAdapterCoreFile_IsOnlyRouterResolutionShim(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "..", "adapters", "cliwrapper", "core.go")
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
	require.NoError(t, err)

	var funcNames []string
	var typeNames []string
	var valueNames []string

	for _, decl := range file.Decls {
		switch typed := decl.(type) {
		case *ast.FuncDecl:
			funcNames = append(funcNames, typed.Name.Name)
		case *ast.GenDecl:
			for _, spec := range typed.Specs {
				switch concrete := spec.(type) {
				case *ast.TypeSpec:
					typeNames = append(typeNames, concrete.Name.Name)
				case *ast.ValueSpec:
					for _, name := range concrete.Names {
						valueNames = append(valueNames, name.Name)
					}
				}
			}
		}
	}

	assert.Equal(t, []string{"resolveWrapperCore"}, funcNames)
	assert.Empty(t, typeNames, "adapter core shim must not redeclare wrapper-owned types")
	assert.Empty(t, valueNames, "adapter core shim must not redeclare wrapper-owned constants or vars")
}
