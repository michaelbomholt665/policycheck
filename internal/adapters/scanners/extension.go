// internal/adapters/scanners/extension.go
package scanners

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"policycheck/internal/ports"
	"policycheck/internal/router"
)

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

// ScanFile executes the external scanners against a single file.
func (a *Adapter) ScanFile(ctx context.Context, root, path string) ([]ports.PolicyFact, error) {
	if filepath.Ext(path) == ".go" {
		return scanGoFile(path)
	}

	ext := filepath.Ext(path)
	runtime, script := resolveScannerScript(ext)
	if runtime == "" {
		return nil, nil
	}

	tempDir, scriptPath, err := setupSingleScannerScript(ext, runtime, script)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	return executeSingleScanner(ctx, scannerContext{
		runtime:    runtime,
		scriptPath: scriptPath,
		root:       root,
		path:       path,
	})
}

func scanGoFile(path string) ([]ports.PolicyFact, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse go file: %w", err)
	}
	return extractGoFunctionFacts(fset, f, path), nil
}

func resolveScannerScript(ext string) (string, []byte) {
	if ext == ".py" {
		return "python", policyScannerPy
	}
	if ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".jsx" {
		return "node", policyScannerJS
	}
	return "", nil
}

func setupSingleScannerScript(ext, runtime string, script []byte) (string, string, error) {
	tempDir, err := os.MkdirTemp("", "scanner-single-*")
	if err != nil {
		return "", "", err
	}

	scriptPath := filepath.Join(tempDir, "scanner"+ext)
	if runtime == "node" {
		scriptPath = filepath.Join(tempDir, "scanner.cjs")
	}

	if err := writeScannerScript(scriptPath, script); err != nil {
		os.RemoveAll(tempDir)
		return "", "", err
	}
	return tempDir, scriptPath, nil
}

type scannerContext struct {
	runtime    string
	scriptPath string
	root       string
	path       string
	walk       ports.WalkProvider
}

func executeSingleScanner(ctx context.Context, sctx scannerContext) ([]ports.PolicyFact, error) {
	cmd := exec.CommandContext(ctx, sctx.runtime, sctx.scriptPath, "--root", sctx.root, "--file", sctx.path)

	cwd, _ := os.Getwd()
	realRoot := sctx.root
	if _, err := os.Stat(filepath.Join(sctx.root, "node_modules")); os.IsNotExist(err) {
		if _, err := os.Stat(filepath.Join(cwd, "node_modules")); err == nil {
			realRoot = cwd
		} else {
			gitRoot, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
			if err == nil {
				realRoot = strings.TrimSpace(string(gitRoot))
			}
		}
	}
	cmd.Env = scannerCommandEnv(realRoot, sctx.runtime)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	facts := []ports.PolicyFact{}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		var fact ports.PolicyFact
		if err := json.Unmarshal(scanner.Bytes(), &fact); err == nil && fact.Kind == "function_quality_fact" {
			facts = append(facts, fact)
		}
	}

	_ = cmd.Wait()
	return facts, nil
}

// RunScanners executes the external scanners against the provided root directory.
func (a *Adapter) RunScanners(ctx context.Context, root string) ([]ports.PolicyFact, error) {
	// Resolve walk provider from router
	rawWalk, err := router.RouterResolveProvider(router.PortWalk)
	if err != nil {
		return nil, fmt.Errorf("resolve walk provider: %w", err)
	}
	walkProvider, ok := rawWalk.(ports.WalkProvider)
	if !ok {
		return nil, fmt.Errorf("walk provider does not satisfy contract")
	}

	tempDir, err := createScannerTempDir(root)
	if err != nil {
		return nil, fmt.Errorf("create temp dir for scanners: %w", err)
	}
	defer os.RemoveAll(tempDir)

	pyPath := filepath.Join(tempDir, "policy_scanner.py")
	if err := writeScannerScript(pyPath, policyScannerPy); err != nil {
		return nil, fmt.Errorf("write python scanner script: %w", err)
	}

	jsPath := filepath.Join(tempDir, "policy_scanner.cjs")
	if err := writeScannerScript(jsPath, policyScannerJS); err != nil {
		return nil, fmt.Errorf("write javascript scanner script: %w", err)
	}

	facts := []ports.PolicyFact{}

	// 1. Run Go scanner (internal)
	goFacts, err := runGoScanner(root, walkProvider)
	if err != nil {
		return nil, fmt.Errorf("run go scanner: %w", err)
	}
	facts = append(facts, goFacts...)

	// 2. Run Python scanner
	pyFacts, err := runScanner(ctx, scannerContext{
		root:       root,
		runtime:    "python",
		scriptPath: pyPath,
		walk:       walkProvider,
	})
	if err != nil {
		log.Printf("non-fatal python scanner failure: %v", err)
	} else {
		facts = append(facts, pyFacts...)
	}

	// 3. Run JS scanner
	jsFacts, err := runScanner(ctx, scannerContext{
		root:       root,
		runtime:    "node",
		scriptPath: jsPath,
		walk:       walkProvider,
	})
	if err != nil {
		log.Printf("non-fatal node scanner failure: %v", err)
	} else {
		facts = append(facts, jsFacts...)
	}

	return facts, nil
}

func runGoScanner(root string, walk ports.WalkProvider) ([]ports.PolicyFact, error) {
	var facts []ports.PolicyFact
	fset := token.NewFileSet()

	err := walk.WalkDirectoryTree(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk %s: %w", path, err)
		}
		if d.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}

		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse go file %s: %w", path, err)
		}

		rel := toSlashRel(root, path)

		facts = append(facts, extractGoFunctionFacts(fset, f, rel)...)
		return nil
	})

	return facts, err
}

func extractGoFunctionFacts(fset *token.FileSet, f *ast.File, rel string) []ports.PolicyFact {
	var facts []ports.PolicyFact
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		start := fset.Position(fn.Pos()).Line
		end := fset.Position(fn.End()).Line

		fact := ports.PolicyFact{
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

func runScanner(ctx context.Context, sctx scannerContext) ([]ports.PolicyFact, error) {
	files, err := gatherScannerFiles(sctx.root, sctx.runtime, sctx.walk)
	if err != nil || len(files) == 0 {
		return nil, nil
	}

	cmd := exec.CommandContext(ctx, sctx.runtime, sctx.scriptPath, "--root", sctx.root, "--file")
	cmd.Args = append(cmd.Args, files...)
	cmd.Dir = sctx.root
	cmd.Env = scannerCommandEnv(sctx.root, sctx.runtime)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	facts := []ports.PolicyFact{}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		var fact ports.PolicyFact
		if err := json.Unmarshal(scanner.Bytes(), &fact); err != nil {
			return nil, fmt.Errorf("decode %s scanner output: %w", sctx.runtime, err)
		}
		if fact.Kind == "function_quality_fact" {
			facts = append(facts, fact)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s output: %w", sctx.runtime, err)
	}

	if err := cmd.Wait(); err != nil {
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText != "" {
			return nil, fmt.Errorf("wait for %s scanner: %w: %s", sctx.runtime, err, stderrText)
		}

		return nil, fmt.Errorf("wait for %s scanner: %w", sctx.runtime, err)
	}

	return facts, nil
}

func createScannerTempDir(root string) (string, error) {
	baseDir := filepath.Join(root, ".policycheck", "scripts")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", fmt.Errorf("create scanner base dir %s: %w", baseDir, err)
	}

	tempDir, err := os.MkdirTemp(baseDir, "scanner-*")
	if err != nil {
		return "", fmt.Errorf("create scanner temp dir in %s: %w", baseDir, err)
	}

	return tempDir, nil
}

func scannerCommandEnv(root, runtime string) []string {
	env := os.Environ()
	if runtime != "node" {
		return env
	}

	nodeModules := filepath.Join(root, "node_modules")
	return append(env, "NODE_PATH="+nodeModules)
}

func writeScannerScript(path string, content []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("open %s for writing: %w", path, err)
	}

	if _, err := file.Write(content); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return fmt.Errorf("write %s: %w (close: %v)", path, err, closeErr)
		}

		return fmt.Errorf("write %s: %w", path, err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}

	return nil
}

func gatherScannerFiles(root, runtime string, walk ports.WalkProvider) ([]string, error) {
	files := []string{}
	err := walk.WalkDirectoryTree(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk %s: %w", path, err)
		}
		if d.IsDir() {
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

func toSlashRel(root, target string) string {
	if !filepath.IsAbs(target) {
		return normalizePolicyPath(target)
	}

	rel, err := filepath.Rel(root, target)
	if err != nil {
		return normalizePolicyPath(target)
	}

	return normalizePolicyPath(rel)
}

func normalizePolicyPath(value string) string {
	cleaned := filepath.ToSlash(filepath.Clean(value))
	if cleaned == "." {
		return ""
	}

	return strings.TrimPrefix(cleaned, "./")
}
