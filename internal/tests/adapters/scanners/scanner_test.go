package scanners_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"policycheck/internal/adapters/scanners"
)

type scannerFact struct {
	Kind       string `json:"kind"`
	Language   string `json:"language"`
	FilePath   string `json:"file_path"`
	SymbolName string `json:"symbol_name"`
	LineNumber int    `json:"line_number"`
	EndLine    int    `json:"end_line"`
	Complexity int    `json:"complexity"`
	ParamCount int    `json:"param_count"`
	SymbolKind string `json:"symbol_kind"`
}

func TestEmbeddedPolicyGateDefaultTemplate(t *testing.T) {
	t.Parallel()

	template := scanners.EmbeddedPolicyGateDefaultTemplate()
	require.NotEmpty(t, template)
	assert.Contains(t, string(template), "# Default policy-gate configuration for policycheck.")
}

func TestPythonScannerCountsKeywordOnlyAndVariadicParameters(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filePath := filepath.Join(root, "sample.py")
	content := "class Example:\n" +
		"    def method(self, value, /, extra, *args, flag=False, **kwargs):\n" +
		"        if value and flag:\n" +
		"            return 1\n" +
		"        return 0\n"
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o644))

	facts := runScannerCommand(
		t,
		"python",
		[]string{
			scannerScriptPath(t, "policy_scanner.py"),
			"--file", filePath,
			"--root", root,
		},
	)

	require.Len(t, facts, 1)
	assert.Equal(t, "python", facts[0].Language)
	assert.Equal(t, "method", facts[0].SymbolKind)
	assert.Equal(t, "method", facts[0].SymbolName)
	assert.Equal(t, 6, facts[0].ParamCount)
	assert.Equal(t, 3, facts[0].Complexity)
}

func TestTypeScriptScannerReportsArrowFunctionsAndMethodComplexity(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filePath := filepath.Join(root, "sample.ts")
	content := "class Example {\n" +
		"  method(value: boolean, flag: boolean) {\n" +
		"    if (value && flag) {\n" +
		"      return 1;\n" +
		"    }\n" +
		"    return 0;\n" +
		"  }\n" +
		"}\n" +
		"const helper = (value: boolean, flag: boolean, extra: boolean) => {\n" +
		"  if (value || flag) {\n" +
		"    return extra ? 1 : 2;\n" +
		"  }\n" +
		"  return 0;\n" +
		"};\n"
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o644))

	facts := runScannerCommand(
		t,
		"node",
		[]string{
			scannerScriptPath(t, "policy_scanner.cjs"),
			"--file", filePath,
			"--root", root,
		},
	)

	require.Len(t, facts, 2)
	factsByName := make(map[string]scannerFact, len(facts))
	for _, fact := range facts {
		factsByName[fact.SymbolName] = fact
	}

	methodFact, ok := factsByName["method"]
	require.True(t, ok)
	assert.Equal(t, "method", methodFact.SymbolKind)
	assert.Equal(t, 2, methodFact.ParamCount)
	assert.Equal(t, 3, methodFact.Complexity)

	helperFact, ok := factsByName["helper"]
	require.True(t, ok)
	assert.Equal(t, "function", helperFact.SymbolKind)
	assert.Equal(t, 3, helperFact.ParamCount)
	assert.Equal(t, 4, helperFact.Complexity)
}

func runScannerCommand(t *testing.T, runtimeName string, args []string) []scannerFact {
	t.Helper()

	cmd := exec.Command(runtimeName, args...)
	if runtimeName == "node" {
		repoRoot := repoRootPath(t)
		cmd.Env = append(os.Environ(), "NODE_PATH="+filepath.Join(repoRoot, "node_modules"))
		cmd.Dir = repoRoot
	}

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))

	scanner := bufio.NewScanner(bytes.NewReader(output))
	facts := make([]scannerFact, 0)
	for scanner.Scan() {
		var fact scannerFact
		if err := json.Unmarshal(scanner.Bytes(), &fact); err != nil {
			require.NoError(t, err, fmt.Sprintf("line=%q", scanner.Text()))
		}
		if fact.Kind == "function_quality_fact" {
			facts = append(facts, fact)
		}
	}
	require.NoError(t, scanner.Err())

	return facts
}

func scannerScriptPath(t *testing.T, name string) string {
	t.Helper()

	return filepath.Join(repoRootPath(t), "internal", "adapters", "scanners", name)
}

func repoRootPath(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok)

	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", "..", ".."))
}
