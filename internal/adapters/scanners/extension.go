// internal/adapters/scanners/extension.go
package scanners

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"policycheck/internal/policycheck/types"
	"policycheck/internal/ports"
	"policycheck/internal/router"
)

//go:embed policy_scanner.py
var policyScannerPy []byte

//go:embed policy_scanner.cjs
var policyScannerJS []byte

// Extension implements router.Extension for the scanner adapter.
type Extension struct{}

// Required returns true - scanner capability is mandatory.
func (e *Extension) Required() bool { return true }

// Consumes reports that the scanner extension depends on the walk provider.
func (e *Extension) Consumes() []router.PortName {
	return []router.PortName{router.PortWalk}
}

// Provides returns the ports this extension registers.
func (e *Extension) Provides() []router.PortName { return []router.PortName{router.PortScanner} }

// RouterProvideRegistration registers the scanner provider.
func (e *Extension) RouterProvideRegistration(reg *router.Registry) error {
	return reg.RouterRegisterProvider(router.PortScanner, &Adapter{})
}

// ExtensionInstance returns the extension instance.
func ExtensionInstance() router.Extension {
	return &Extension{}
}

// Adapter implements the ports.ScannerProvider interface.
type Adapter struct{}

// RunScanners executes the external scanners against the provided root directory.
func (a *Adapter) RunScanners(ctx context.Context, root string) ([]types.PolicyFact, error) {
	// Resolve walk provider from router
	rawWalk, err := router.RouterResolveProvider(router.PortWalk)
	if err != nil {
		return nil, fmt.Errorf("resolve walk provider: %w", err)
	}
	walkProvider, ok := rawWalk.(ports.WalkProvider)
	if !ok {
		return nil, fmt.Errorf("walk provider does not satisfy contract")
	}

	tempDir, err := os.MkdirTemp("", "policycheck-scanners-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir for scanners: %w", err)
	}
	defer os.RemoveAll(tempDir)

	pyPath := filepath.Join(tempDir, "policy_scanner.py")
	if err := os.WriteFile(pyPath, policyScannerPy, 0o755); err != nil {
		return nil, fmt.Errorf("write py scanner: %w", err)
	}

	jsPath := filepath.Join(tempDir, "policy_scanner.cjs")
	if err := os.WriteFile(jsPath, policyScannerJS, 0o755); err != nil {
		return nil, fmt.Errorf("write js scanner: %w", err)
	}

	facts := []types.PolicyFact{}

	// 1. Run Go scanner (internal)
	goFacts, err := runGoScanner(root, walkProvider)
	if err == nil {
		facts = append(facts, goFacts...)
	}

	// 2. Run Python scanner
	pyFacts, err := runScanner(ctx, root, "python", pyPath, walkProvider)
	if err == nil {
		facts = append(facts, pyFacts...)
	}

	// 3. Run JS scanner
	jsFacts, err := runScanner(ctx, root, "node", jsPath, walkProvider)
	if err == nil {
		facts = append(facts, jsFacts...)
	}

	return facts, nil
}

func runGoScanner(root string, walk ports.WalkProvider) ([]types.PolicyFact, error) {
	var facts []types.PolicyFact
	fset := token.NewFileSet()

	err := walk.WalkDirectoryTree(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}

		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)

		facts = append(facts, extractGoFunctionFacts(fset, f, rel)...)
		return nil
	})

	return facts, err
}

func extractGoFunctionFacts(fset *token.FileSet, f *ast.File, rel string) []types.PolicyFact {
	var facts []types.PolicyFact
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		start := fset.Position(fn.Pos()).Line
		end := fset.Position(fn.End()).Line

		fact := types.PolicyFact{
			Kind:       "function_quality_fact",
			Language:   "go",
			FilePath:   rel,
			SymbolName: fn.Name.Name,
			LineNumber: start,
			EndLine:    end,
			Complexity: calculateComplexity(fn),
			ParamCount: fn.Type.Params.NumFields(),
			SymbolKind: "function",
		}

		maxRepeatedGuards := 0
		for _, count := range analyzeRepeatedNilGuards(fn) {
			if count > maxRepeatedGuards {
				maxRepeatedGuards = count
			}
		}
		fact.RepeatedNilGuards = maxRepeatedGuards

		if fn.Recv != nil {
			fact.SymbolKind = "method"
		}
		facts = append(facts, fact)
		return true
	})
	return facts
}

func runScanner(ctx context.Context, root, runtime, scriptPath string, walk ports.WalkProvider) ([]types.PolicyFact, error) {
	files, err := gatherScannerFiles(root, runtime, walk)
	if err != nil || len(files) == 0 {
		return nil, nil
	}

	cmd := exec.CommandContext(ctx, runtime, scriptPath, "--root", root, "--file")
	cmd.Args = append(cmd.Args, files...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	facts := []types.PolicyFact{}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		var fact types.PolicyFact
		if err := json.Unmarshal(scanner.Bytes(), &fact); err == nil && fact.Kind == "function_quality_fact" {
			facts = append(facts, fact)
		}
	}

	_ = cmd.Wait()
	return facts, nil
}

func gatherScannerFiles(root, runtime string, walk ports.WalkProvider) ([]string, error) {
	files := []string{}
	err := walk.WalkDirectoryTree(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if (runtime == "python" && ext == ".py") || (runtime == "node" && (ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".jsx")) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
