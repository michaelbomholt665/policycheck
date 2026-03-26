package hygiene_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"policycheck/internal/app"
	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/core/hygiene"
	"policycheck/internal/policycheck/types"
	"policycheck/internal/ports"
	"policycheck/internal/router"
)

func TestCheckDocumentation_StrictHeaders(t *testing.T) {
	bootPolicycheckForDocumentationTest(t)

	root := t.TempDir()

	writeDocTestFile(t, root, "internal/wrong.go", `// internal/not-wrong.go
// Package-level summary line one.
// Package-level summary line two.

package wrong
`)
	writeDocTestFile(t, root, "internal/short.ts", `// internal/short.ts
// only one line

export function short(): void {}
`)
	writeDocTestFile(t, root, "scripts/no_shebang.py", `# scripts/no_shebang.py
# Summary line one.
# Summary line two.

def run_task():
    """Run task.

    Returns
    -------
    None
    """
    return None
`)
	writeDocTestFile(t, root, "internal/plain.py", `# internal/plain.py
# Summary line one.
# Summary line two.

def helper():
    """Helper summary.

    Returns
    -------
    None
    """
    return None
`)

	cfg := config.PolicyConfig{
		Documentation: config.PolicyDocumentationConfig{
			Enabled:              true,
			Level:                "strict",
			ScanRoots:            []string{"internal", "scripts"},
			PythonStyle:          "numpy",
			EnforceHeaders:       true,
			RequireShebangPython: true,
			PythonShebangRoots:   []string{"scripts"},
		},
	}

	violations := hygiene.CheckDocumentation(context.Background(), root, cfg)
	assert.Len(t, violations, 3)

	assertViolationMessage(t, violations, "internal/wrong.go", "file header path is incorrect", "level=strict", `expected "internal/wrong.go" on line 1`, "go_style=")
	assertViolationMessage(t, violations, "internal/short.ts", "module description has 1 line(s)", "level=strict", "expected 2-5 comment lines immediately after the path header", "typescript_style=")
	assertViolationMessage(t, violations, "scripts/no_shebang.py", "missing required shebang", "level=strict", `expected "#!/usr/bin/env python3" on line 1`, "python_style=numpy")
	assertNoViolationForFile(t, violations, "internal/plain.py")
}

func TestCheckDocumentation_LooseFunctionsAcrossLanguages(t *testing.T) {
	bootPolicycheckForDocumentationTest(t)

	root := t.TempDir()

	writeDocTestFile(t, root, "internal/undoc.go", `// internal/undoc.go
// Summary line one.
// Summary line two.

package undoc

func Undoc() {}
`)
	writeDocTestFile(t, root, "scripts/undoc.py", `# scripts/undoc.py
# Summary line one.
# Summary line two.

def undoc():
    pass
`)
	writeDocTestFile(t, root, "internal/undoc.ts", `// internal/undoc.ts
// Summary line one.
// Summary line two.

export function undoc(value: number): number {
  return value;
}
`)

	cfg := config.PolicyConfig{
		Documentation: config.PolicyDocumentationConfig{
			Enabled:          true,
			Level:            "loose",
			ScanRoots:        []string{"internal", "scripts"},
			GoStyle:          "google",
			PythonStyle:      "standard",
			TypeScriptStyle:  "standard",
			EnforceFunctions: true,
		},
	}

	violations := hygiene.CheckDocumentation(context.Background(), root, cfg)
	assert.Len(t, violations, 3)

	assertViolationMessage(t, violations, "internal/undoc.go", `function "Undoc" is missing documentation`, "level=loose", "go_style=google", "expected a doc comment immediately above the function")
	assertViolationMessage(t, violations, "scripts/undoc.py", `function "undoc" is missing documentation`, "level=loose", "python_style=standard", "expected attached documentation immediately above the function")
	assertViolationMessage(t, violations, "internal/undoc.ts", `function "undoc" is missing documentation`, "level=loose", "typescript_style=standard", "expected attached documentation immediately above the function")
}

func TestCheckDocumentation_StrictStyleDiagnostics(t *testing.T) {
	bootPolicycheckForDocumentationTest(t)

	root := t.TempDir()

	writeDocTestFile(t, root, "internal/bad_go.go", `// internal/bad_go.go
// Summary line one.
// Summary line two.

package badgo

// This function handles work.
func BuildReport() {}
`)
	writeDocTestFile(t, root, "scripts/bad_numpy.py", `#!/usr/bin/env python3
# scripts/bad_numpy.py
# Summary line one.
# Summary line two.

def build_report(value):
    """Build a report.

    Returns
    -------
    int
    """
    return value
`)
	writeDocTestFile(t, root, "internal/bad_ts.ts", `// internal/bad_ts.ts
// Summary line one.
// Summary line two.

// Build a report for the caller.
export function buildReport(config: object): object {
  return config;
}
`)

	cfg := config.PolicyConfig{
		Documentation: config.PolicyDocumentationConfig{
			Enabled:              true,
			Level:                "strict",
			ScanRoots:            []string{"internal", "scripts"},
			GoStyle:              "google",
			PythonStyle:          "numpy",
			TypeScriptStyle:      "tsdoc",
			EnforceFunctions:     true,
			RequireShebangPython: true,
			PythonShebangRoots:   []string{"scripts"},
		},
	}

	violations := hygiene.CheckDocumentation(context.Background(), root, cfg)
	assert.Len(t, violations, 3)

	assertViolationMessage(t, violations, "internal/bad_go.go", `function "BuildReport" violates documentation style`, "level=strict", "go_style=google", `expected the summary line to start with "BuildReport"`)
	assertViolationMessage(t, violations, "scripts/bad_numpy.py", `function "build_report" violates documentation style`, "level=strict", "python_style=numpy", `missing required "Parameters" section`)
	assertViolationMessage(t, violations, "internal/bad_ts.ts", `function "buildReport" violates documentation style`, "level=strict", "typescript_style=tsdoc", "expected a /** ... */ documentation block immediately above the function")
}

func TestCheckDocumentation_StrictPythonGoogleAndREST(t *testing.T) {
	bootPolicycheckForDocumentationTest(t)

	root := t.TempDir()

	writeDocTestFile(t, root, "scripts/bad_google.py", `#!/usr/bin/env python3
# scripts/bad_google.py
# Summary line one.
# Summary line two.

def build_report(value):
    """Build a report.

    Returns:
        int: The computed result.
    """
    return value
`)
	writeDocTestFile(t, root, "scripts/bad_rest.py", `#!/usr/bin/env python3
# scripts/bad_rest.py
# Summary line one.
# Summary line two.

def build_rest_report(value):
    """Build a reST report.

    :returns: The computed result.
    """
    return value
`)

	cfg := config.PolicyConfig{
		Documentation: config.PolicyDocumentationConfig{
			Enabled:              true,
			Level:                "strict",
			ScanRoots:            []string{"scripts"},
			PythonStyle:          "google",
			EnforceFunctions:     true,
			RequireShebangPython: true,
			PythonShebangRoots:   []string{"scripts"},
		},
	}

	googleViolations := hygiene.CheckDocumentation(context.Background(), root, cfg)
	assertViolationMessage(t, googleViolations, "scripts/bad_google.py", `function "build_report" violates documentation style`, "python_style=google", `missing required "Args:" section`)

	cfg.Documentation.PythonStyle = "restructuredtext"
	restViolations := hygiene.CheckDocumentation(context.Background(), root, cfg)
	assertViolationMessage(t, restViolations, "scripts/bad_rest.py", `function "build_rest_report" violates documentation style`, "python_style=restructuredtext", `missing :param field for argument "value"`)
}

func TestCheckDocumentation_StrictGoGoogleBlankSeparator(t *testing.T) {
	bootPolicycheckForDocumentationTest(t)

	root := t.TempDir()
	writeDocTestFile(t, root, "internal/bad_separator.go", `// internal/bad_separator.go
// Summary line one.
// Summary line two.

package badseparator

// BuildSummary reports the summary.
// Additional details start immediately without a blank separator.
func BuildSummary() {}
`)

	cfg := config.PolicyConfig{
		Documentation: config.PolicyDocumentationConfig{
			Enabled:          true,
			Level:            "strict",
			ScanRoots:        []string{"internal"},
			GoStyle:          "google",
			EnforceFunctions: true,
		},
	}

	violations := hygiene.CheckDocumentation(context.Background(), root, cfg)
	assertViolationMessage(t, violations, "internal/bad_separator.go", `function "BuildSummary" violates documentation style`, "go_style=google", "expected a blank comment separator line before an additional paragraph")
}

func TestCheckDocumentation_StrictTypeScriptMissingParamTag(t *testing.T) {
	bootPolicycheckForDocumentationTest(t)

	root := t.TempDir()
	writeDocTestFile(t, root, "internal/bad_param.ts", `// internal/bad_param.ts
// Summary line one.
// Summary line two.

/**
 * Build a report for the caller.
 * @returns The report config.
 */
export function buildReport(config: object): object {
  return config;
}
`)

	cfg := config.PolicyConfig{
		Documentation: config.PolicyDocumentationConfig{
			Enabled:          true,
			Level:            "strict",
			ScanRoots:        []string{"internal"},
			TypeScriptStyle:  "tsdoc",
			EnforceFunctions: true,
		},
	}

	violations := hygiene.CheckDocumentation(context.Background(), root, cfg)
	assertViolationMessage(t, violations, "internal/bad_param.ts", `function "buildReport" violates documentation style`, "typescript_style=tsdoc", `missing @param tag for argument "config"`)
}

func TestCheckDocumentation_TypeScriptLooseLineCommentCountsAsDocumentation(t *testing.T) {
	bootPolicycheckForDocumentationTest(t)

	root := t.TempDir()
	writeDocTestFile(t, root, "internal/documented.ts", `// internal/documented.ts
// Summary line one.
// Summary line two.

// Build a report for the caller.
export function buildReport(config: object): object {
  return config;
}
`)

	cfg := config.PolicyConfig{
		Documentation: config.PolicyDocumentationConfig{
			Enabled:          true,
			Level:            "loose",
			ScanRoots:        []string{"internal"},
			TypeScriptStyle:  "presence_only",
			EnforceFunctions: true,
		},
	}

	violations := hygiene.CheckDocumentation(context.Background(), root, cfg)
	assert.Empty(t, violations)
}

func TestCheckDocumentation_UsesRouterResolvedScannerProvider(t *testing.T) {
	router.RouterResetForTest()
	t.Cleanup(router.RouterResetForTest)

	root := t.TempDir()
	pythonPath := filepath.Join(root, "scripts", "doc.py")
	writeDocTestFile(t, root, "scripts/doc.py", `#!/usr/bin/env python3
# scripts/doc.py
# Summary line one.
# Summary line two.

def build_report(value):
    pass
`)

	mockWalk := &documentationMockWalk{paths: []string{pythonPath}}
	mockScanner := &documentationMockScanner{
		facts: []types.PolicyFact{{
			Language:   "python",
			SymbolKind: "function",
			SymbolName: "build_report",
			LineNumber: 5,
			Params:     []string{"value"},
			Docstring:  "",
		}},
	}

	_, err := router.RouterLoadExtensions(nil, []router.Extension{
		&documentationMockExtension{port: router.PortWalk, provider: mockWalk},
		&documentationMockExtension{port: router.PortReadFile, provider: documentationMockReadFile{}},
		&documentationMockExtension{port: router.PortScanner, provider: mockScanner},
	}, context.Background())
	require.NoError(t, err)

	cfg := config.PolicyConfig{
		Documentation: config.PolicyDocumentationConfig{
			Enabled:          true,
			Level:            "loose",
			ScanRoots:        []string{"scripts"},
			PythonStyle:      "standard",
			EnforceFunctions: true,
		},
	}

	violations := hygiene.CheckDocumentation(context.Background(), root, cfg)
	require.True(t, mockScanner.called, "expected documentation check to resolve scanner through router")
	require.Len(t, mockScanner.scanPaths, 1)
	assert.Equal(t, pythonPath, mockScanner.scanPaths[0])
	assertViolationMessage(t, violations, "scripts/doc.py", `function "build_report" is missing documentation`, "python_style=standard")
}

func bootPolicycheckForDocumentationTest(t *testing.T) {
	t.Helper()

	router.RouterResetForTest()
	require.NoError(t, app.BootPolicycheckApp(context.Background()))
}

func writeDocTestFile(t *testing.T, root, relativePath, content string) {
	t.Helper()

	fullPath := filepath.Join(root, filepath.FromSlash(relativePath))
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
	require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
}

func assertViolationMessage(t *testing.T, violations []types.Violation, file string, parts ...string) {
	t.Helper()

	for _, violation := range violations {
		if violation.File != file {
			continue
		}

		for _, part := range parts {
			assert.Contains(t, violation.Message, part)
		}
		return
	}

	t.Fatalf("missing violation for %s", file)
}

func assertNoViolationForFile(t *testing.T, violations []types.Violation, file string) {
	t.Helper()

	for _, violation := range violations {
		if strings.EqualFold(violation.File, file) {
			t.Fatalf("unexpected violation for %s: %s", file, violation.Message)
		}
	}
}

type documentationMockExtension struct {
	port     router.PortName
	provider router.Provider
}

func (m *documentationMockExtension) Required() bool {
	return true
}

func (m *documentationMockExtension) Consumes() []router.PortName {
	return nil
}

func (m *documentationMockExtension) Provides() []router.PortName {
	return []router.PortName{m.port}
}

func (m *documentationMockExtension) RouterProvideRegistration(reg *router.Registry) error {
	return reg.RouterRegisterProvider(m.port, m.provider)
}

type documentationMockWalk struct {
	paths []string
}

func (m *documentationMockWalk) WalkDirectoryTree(root string, walkFn fs.WalkDirFunc) error {
	for _, path := range m.paths {
		if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(root)) {
			continue
		}

		info, err := os.Stat(path)
		if err != nil {
			return err
		}

		entry := fs.FileInfoToDirEntry(info)
		if err := walkFn(path, entry, nil); err != nil {
			return err
		}
	}

	return nil
}

type documentationMockReadFile struct{}

func (documentationMockReadFile) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

type documentationMockScanner struct {
	facts     []ports.PolicyFact
	called    bool
	scanPaths []string
}

func (m *documentationMockScanner) RunScanners(ctx context.Context, root string) ([]ports.PolicyFact, error) {
	return m.facts, nil
}

func (m *documentationMockScanner) ScanFile(ctx context.Context, root, path string) ([]ports.PolicyFact, error) {
	m.called = true
	m.scanPaths = append(m.scanPaths, path)
	return m.facts, nil
}
