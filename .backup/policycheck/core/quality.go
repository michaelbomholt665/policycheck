// internal/policycheck/core/quality.go
// Implements file LOC and function CTX/LOC quality policy checks across all supported languages.

package core

const ScopeProjectRepo = true


import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/embedded"
	"policycheck/internal/policycheck/types"
	"policycheck/internal/policycheck/utils"
)

const (
	policyFactKind           = "function_quality_fact"
	policyScannerPythonName  = "policy_scanner.py"
	policyScannerTSName      = "policy_scanner.cjs"
	policyFactLanguageGo     = "go"
	policyFactLanguagePython = "python"
	policyFactLanguageTS     = "typescript"
	policyScannerTimeout     = 30 * time.Second
)

// CheckCoreFileLOCPolicies validates line-of-code limits for Go source files with CTX-based penalties.
func CheckCoreFileLOCPolicies(root string, cfg config.PolicyConfig) ([]types.Violation, []types.Violation) {
	warnings := []types.Violation{}
	violations := []types.Violation{}

	for _, relBase := range cfg.Paths.FileLOCRoots {
		base := filepath.Join(root, filepath.FromSlash(relBase))
		_ = filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
			result := evaluateFileLOCPolicy(root, base, path, entry, walkErr, cfg)
			if result.Warning != nil {
				warnings = append(warnings, *result.Warning)
			}
			if result.Violation != nil {
				violations = append(violations, *result.Violation)
			}
			return nil
		})
	}
	return warnings, violations
}

// evaluateFileLOCPolicy evaluates a single file against LOC policy thresholds.
func evaluateFileLOCPolicy(root, base, path string, entry fs.DirEntry, walkErr error, cfg config.PolicyConfig) types.FileLOCResult {
	if walkErr != nil {
		if os.IsNotExist(walkErr) && path == base {
			return types.FileLOCResult{}
		}
		return types.FileLOCResult{Violation: &types.Violation{Path: path, Message: fmt.Sprintf("error walking directory for LOC scan: %v", walkErr)}}
	}
	if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
		return types.FileLOCResult{}
	}

	rel := utils.RelOrAbs(root, path)
	if shouldIgnoreLOCPolicies(rel, cfg) {
		return types.FileLOCResult{}
	}
	return computeFileLOCResult(path, rel, cfg)
}

// computeFileLOCResult calculates the LOC for a file and determines if it violates policy thresholds.
func computeFileLOCResult(path, rel string, cfg config.PolicyConfig) types.FileLOCResult {
	content, err := os.ReadFile(path)
	if err != nil {
		return types.FileLOCResult{Violation: &types.Violation{Path: rel, Message: fmt.Sprintf("unable to read go file: %v", err)}}
	}
	fileNode, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
	if err != nil {
		return types.FileLOCResult{Violation: &types.Violation{Path: rel, Message: fmt.Sprintf("unable to parse go file for LOC policy: %v", err)}}
	}

	lineCount := utils.CountLines(content)
	warnCTXCount, hardCTXCount := countCTXHeavyFunctions(fileNode, cfg)
	warnThreshold, maxThreshold := AdjustedFileLOCThresholds(cfg, warnCTXCount, hardCTXCount)

	if lineCount > maxThreshold {
		return types.FileLOCResult{Violation: &types.Violation{Path: rel, Message: formatFileLOCBreachMessage(cfg, lineCount, maxThreshold, warnCTXCount, hardCTXCount, true)}}
	}
	if lineCount > warnThreshold {
		return types.FileLOCResult{Warning: &types.Violation{Path: rel, Message: formatFileLOCBreachMessage(cfg, lineCount, warnThreshold, warnCTXCount, hardCTXCount, false)}}
	}
	return types.FileLOCResult{}
}

// countCTXHeavyFunctions counts functions exceeding CTX warning and hard thresholds in a file.
func countCTXHeavyFunctions(fileNode *ast.File, cfg config.PolicyConfig) (int, int) {
	warnCount := 0
	hardCount := 0
	ast.Inspect(fileNode, func(node ast.Node) bool {
		switch fn := node.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			ctx := utils.CalculateComplexity(fn)
			if ctx >= cfg.FunctionQuality.MildCTXMin {
				warnCount++
			}
			if ctx >= cfg.FileSize.MaxPenaltyCTXThreshold {
				hardCount++
			}
		}
		return true
	})
	return warnCount, hardCount
}

// AdjustedFileLOCThresholds computes dynamic LOC thresholds based on function complexity penalties.
func AdjustedFileLOCThresholds(cfg config.PolicyConfig, warnCTXCount, hardCTXCount int) (int, int) {
	warnPenalty := warnCTXCount * cfg.FileSize.WarnPenaltyPerCTXFunction
	hardPenalty := hardCTXCount * cfg.FileSize.MaxPenaltyPerCTXFunction
	warnThreshold := utils.MaxInt(cfg.FileSize.MinWarnLOC, cfg.FileSize.WarnLOC-warnPenalty)
	maxThreshold := utils.MaxInt(cfg.FileSize.MinMaxLOC, cfg.FileSize.MaxLOC-hardPenalty)
	if maxThreshold-warnThreshold < cfg.FileSize.MinWarnToMaxGap {
		maxThreshold = warnThreshold + cfg.FileSize.MinWarnToMaxGap
	}
	return warnThreshold, maxThreshold
}

// formatFileLOCBreachMessage creates a human-readable message for LOC policy breaches.
func formatFileLOCBreachMessage(cfg config.PolicyConfig, lineCount, threshold, warnCTXCount, hardCTXCount int, hard bool) string {
	kind := "warn over"
	if hard {
		kind = "hard max"
	}
	if warnCTXCount == 0 && hardCTXCount == 0 {
		return fmt.Sprintf("file has %d LOC (%s %d)", lineCount, kind, threshold)
	}
	return fmt.Sprintf(
		"file has %d LOC (%s %d after CTX penalties: %d function(s) at CTX>=%d, %d function(s) at CTX>=%d)",
		lineCount, kind, threshold, warnCTXCount, cfg.FunctionQuality.MildCTXMin, hardCTXCount, cfg.FileSize.MaxPenaltyCTXThreshold,
	)
}

// shouldIgnoreLOCPolicies checks if a file path should be excluded from LOC policy checks.
func shouldIgnoreLOCPolicies(rel string, cfg config.PolicyConfig) bool {
	return utils.HasPrefix(rel, cfg.Paths.LOCIgnorePrefixes)
}

// CheckCoreFunctionLOCPolicies coordinates scanning of function quality facts across all supported languages.
func CheckCoreFunctionLOCPolicies(root string, cfg config.PolicyConfig, scannerBytes types.ScannerBytes) ([]types.Violation, []types.FunctionQualityWarning, []types.Violation) {
	scannerIssues := []types.Violation{}
	goFacts, goWarnings, goIssues := collectGoFunctionFacts(root, cfg)
	scannerIssues = append(scannerIssues, goIssues...)

	assets, cleanup, err := embedded.MaterializeScannerAssets(scannerBytes)
	if err != nil {
		scannerIssues = append(scannerIssues, types.Violation{Message: fmt.Sprintf("unable to prepare embedded policy scanners: %v", err)})
		functionWarnings, functionViolations := evaluateFunctionQualityFacts(cfg, goFacts)
		functionWarnings = append(functionWarnings, goWarnings...)
		return scannerIssues, functionWarnings, functionViolations
	}
	defer cleanup()

	externalFacts, externalIssues := collectExternalFunctionFacts(root, cfg, assets)
	scannerIssues = append(scannerIssues, externalIssues...)
	allFacts := append(goFacts, externalFacts...)
	functionWarnings, functionViolations := evaluateFunctionQualityFacts(cfg, allFacts)
	functionWarnings = append(functionWarnings, goWarnings...)
	return scannerIssues, functionWarnings, functionViolations
}

// collectGoFunctionFacts extracts function quality facts from Go source files.
func collectGoFunctionFacts(root string, cfg config.PolicyConfig) ([]types.PolicyFact, []types.FunctionQualityWarning, []types.Violation) {
	facts := []types.PolicyFact{}
	warnings := []types.FunctionQualityWarning{}
	issues := []types.Violation{}
	fset := token.NewFileSet()

	for _, relBase := range cfg.Paths.FunctionQualityRoots {
		base := filepath.Join(root, filepath.FromSlash(relBase))
		_ = filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel := utils.RelOrAbs(root, path)
			if shouldIgnoreLOCPolicies(rel, cfg) {
				return nil
			}
			fileNode, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				issues = append(issues, types.Violation{Path: rel, Message: fmt.Sprintf("unable to parse go file for function quality policy: %v", err)})
				return nil
			}
			fileFacts, fileWarnings := extractGoFunctionFacts(fset, fileNode, rel, cfg)
			facts = append(facts, fileFacts...)
			warnings = append(warnings, fileWarnings...)
			return nil
		})
	}
	return facts, warnings, issues
}

// extractGoFunctionFacts parses Go functions from an AST and produces quality facts.
func extractGoFunctionFacts(fset *token.FileSet, fileNode *ast.File, rel string, cfg config.PolicyConfig) ([]types.PolicyFact, []types.FunctionQualityWarning) {
	facts := []types.PolicyFact{}
	warnings := []types.FunctionQualityWarning{}

	ast.Inspect(fileNode, func(node ast.Node) bool {
		var name string
		var symbolKind string
		switch fn := node.(type) {
		case *ast.FuncDecl:
			name = fn.Name.Name
			symbolKind = goPolicySymbolKind(fn)
		case *ast.FuncLit:
			name = "anonymous"
			symbolKind = "function"
		default:
			return true
		}

		start := fset.Position(node.Pos()).Line
		end := fset.Position(node.End()).Line
		fact := types.PolicyFact{
			Kind: policyFactKind, Language: policyFactLanguageGo, Path: rel,
			Name: name, Line: start, EndLine: end, LOC: end - start + 1,
			CTX: utils.CalculateComplexity(node), SymbolKind: symbolKind,
		}
		facts = append(facts, fact)

		if fact.CTX >= cfg.FunctionQuality.MildCTXMin {
			for ident, repeats := range utils.AnalyzeRepeatedNilGuards(node) {
				if repeats >= cfg.FunctionQuality.NilGuardRepeatWarnCount {
					warnings = append(warnings, types.FunctionQualityWarning{
						Path: rel, Function: name, Level: types.FunctionQualityLevelElevated,
						Message: fmt.Sprintf("function %s repeats plain nil-guard checks on %s %d times; CTX may be inflated by distant repeated guards", name, ident, repeats),
					})
				}
			}
		}
		return true
	})
	return facts, warnings
}

// collectExternalFunctionFacts runs external policy scanners (Python/TypeScript) to gather function quality data.
func collectExternalFunctionFacts(root string, cfg config.PolicyConfig, assets types.PolicyScannerAssets) ([]types.PolicyFact, []types.Violation) {
	facts := []types.PolicyFact{}
	issues := []types.Violation{}

	pythonFiles := collectPolicySourceFiles(root, cfg, []string{".py"})
	tsFiles := collectPolicySourceFiles(root, cfg, []string{".ts", ".tsx"})

	pythonFacts, pythonIssues := scanExternalPolicyFiles(root, pythonFiles, policyFactLanguagePython, assets.Python)
	facts = append(facts, pythonFacts...)
	issues = append(issues, pythonIssues...)

	tsFacts, tsIssues := scanExternalPolicyFiles(root, tsFiles, policyFactLanguageTS, assets.TS)
	facts = append(facts, tsFacts...)
	issues = append(issues, tsIssues...)

	return facts, issues
}

// collectPolicySourceFiles gathers source files matching given extensions from configured roots.
func collectPolicySourceFiles(root string, cfg config.PolicyConfig, extensions []string) []string {
	seen := make(map[string]struct{})
	files := []string{}

	for _, relBase := range cfg.Paths.FunctionQualityRoots {
		base := filepath.Join(root, filepath.FromSlash(relBase))
		_ = filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() || !hasFileExtension(path, extensions) {
				return nil
			}
			rel := utils.RelOrAbs(root, path)
			if shouldIgnoreLOCPolicies(rel, cfg) {
				return nil
			}
			if _, ok := seen[path]; ok {
				return nil
			}
			seen[path] = struct{}{}
			files = append(files, path)
			return nil
		})
	}
	sort.Strings(files)
	return files
}

// scanExternalPolicyFiles invokes an external policy scanner for the specified language and files.
func scanExternalPolicyFiles(root string, files []string, language, scriptPath string) ([]types.PolicyFact, []types.Violation) {
	if len(files) == 0 {
		return nil, nil
	}
	commandName, commandArgs := buildPolicyScannerCommand(language, scriptPath)
	if commandName == "" {
		return nil, []types.Violation{{Message: fmt.Sprintf("%s policy scanner unavailable; skipping %s files because the required runtime is not installed", language, language)}}
	}
	facts, err := runPolicyScannerCommand(root, files, commandName, commandArgs)
	if err != nil {
		return nil, []types.Violation{{Path: root, Message: fmt.Sprintf("%s policy scanner failed: %v", language, err)}}
	}
	return facts, nil
}

// buildPolicyScannerCommand constructs the command and arguments to run a policy scanner.
func buildPolicyScannerCommand(language, scriptPath string) (string, []string) {
	switch language {
	case policyFactLanguagePython:
		candidates := []string{"python3", "python"}
		if runtime.GOOS == "windows" {
			candidates = []string{"python", "python3"}
		}
		commandName := findFirstAvailableCommand(candidates)
		if commandName == "" {
			return "", nil
		}
		return commandName, []string{scriptPath}
	case policyFactLanguageTS:
		commandName := findFirstAvailableCommand([]string{"node"})
		if commandName == "" {
			return "", nil
		}
		return commandName, []string{scriptPath}
	default:
		return "", nil
	}
}

// runPolicyScannerCommand executes an external policy scanner with given files and arguments.
func runPolicyScannerCommand(root string, paths []string, commandName string, baseArgs []string) ([]types.PolicyFact, error) {
	args := append([]string{}, baseArgs...)
	args = append(args, "--file")
	args = append(args, paths...)
	args = append(args, "--root", root)

	ctx, cancel := context.WithTimeout(context.Background(), policyScannerTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, commandName, args...)
	cmd.Env = policyScannerEnvironment(commandName)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("scanner command timed out after %s", policyScannerTimeout)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return parsePolicyFactsOutput(output)
}

// parsePolicyFactsOutput parses JSON output from a policy scanner into PolicyFact structs.
func parsePolicyFactsOutput(output []byte) ([]types.PolicyFact, error) {
	facts := []types.PolicyFact{}
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var fact types.PolicyFact
		if err := json.Unmarshal([]byte(line), &fact); err != nil {
			return nil, fmt.Errorf("decode policy fact: %w", err)
		}
		if fact.Kind != policyFactKind {
			return nil, fmt.Errorf("unexpected policy fact kind %q", fact.Kind)
		}
		facts = append(facts, fact)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan policy facts: %w", err)
	}
	return facts, nil
}

// policyScannerEnvironment returns the environment variables needed for running a policy scanner.
func policyScannerEnvironment(commandName string) []string {
	env := os.Environ()
	if commandName != "node" {
		return env
	}
	nodeModulesDir := findLocalNodeModulesDir()
	if nodeModulesDir == "" {
		return env
	}
	return upsertEnvironmentValue(env, "NODE_PATH", nodeModulesDir)
}

// findLocalNodeModulesDir searches for a local node_modules directory from the current working directory.
func findLocalNodeModulesDir() string {
	workingDir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for dir := workingDir; ; dir = filepath.Dir(dir) {
		nodeModulesDir := filepath.Join(dir, "node_modules")
		if info, statErr := os.Stat(nodeModulesDir); statErr == nil && info.IsDir() {
			return nodeModulesDir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
	}
}

// upsertEnvironmentValue adds or updates an environment variable in the given environment slice.
func upsertEnvironmentValue(env []string, key, value string) []string {
	prefix := key + "="
	for idx, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			result := append([]string{}, env...)
			result[idx] = prefix + value
			return result
		}
	}
	return append(env, prefix+value)
}

// evaluateFunctionQualityFacts evaluates function quality facts against policy thresholds.
func evaluateFunctionQualityFacts(cfg config.PolicyConfig, facts []types.PolicyFact) ([]types.FunctionQualityWarning, []types.Violation) {
	warnings := []types.FunctionQualityWarning{}
	violations := []types.Violation{}

	for _, fact := range facts {
		if fact.CTX >= cfg.FunctionQuality.ErrorCTXMin || fact.LOC >= cfg.FunctionQuality.MaxLOC || (fact.CTX >= cfg.FunctionQuality.ErrorCTXAndLOCCTX && fact.LOC >= cfg.FunctionQuality.ErrorCTXAndLOCLOC) {
			violations = append(violations, types.Violation{
				Path:    fact.Path,
				Message: fmt.Sprintf("%s %s quality failure: CTX=%d (max %d), LOC=%d (max %d); immediate refactor required", policySubjectLabel(fact), fact.Name, fact.CTX, cfg.FunctionQuality.ErrorCTXMin, fact.LOC, cfg.FunctionQuality.MaxLOC),
			})
			continue
		}
		if warning, ok := BuildFunctionQualityWarning(cfg, fact); ok {
			warnings = append(warnings, warning)
		}
	}
	return warnings, violations
}

// BuildFunctionQualityWarning constructs a warning if a function exceeds quality thresholds.
func BuildFunctionQualityWarning(cfg config.PolicyConfig, fact types.PolicyFact) (types.FunctionQualityWarning, bool) {
	subject := policySubjectLabel(fact)
	if fact.CTX >= cfg.FunctionQuality.ImmediateRefactorCTXMin {
		return types.FunctionQualityWarning{Path: fact.Path, Function: fact.Name, Level: types.FunctionQualityLevelImmediate, Message: fmt.Sprintf("%s %s quality warning: CTX=%d, LOC=%d (immediate refactor recommended)", subject, fact.Name, fact.CTX, fact.LOC)}, true
	}
	if fact.CTX >= cfg.FunctionQuality.ElevatedCTXMin {
		return types.FunctionQualityWarning{Path: fact.Path, Function: fact.Name, Level: types.FunctionQualityLevelElevated, Message: fmt.Sprintf("%s %s quality warning: CTX=%d, LOC=%d (elevated warning)", subject, fact.Name, fact.CTX, fact.LOC)}, true
	}
	if fact.CTX >= cfg.FunctionQuality.MildCTXMin {
		return types.FunctionQualityWarning{Path: fact.Path, Function: fact.Name, Level: types.FunctionQualityLevelMild, Message: fmt.Sprintf("%s %s quality warning: CTX=%d, LOC=%d (mild warning)", subject, fact.Name, fact.CTX, fact.LOC)}, true
	}
	if fact.LOC >= cfg.FunctionQuality.WarnLOC {
		return types.FunctionQualityWarning{Path: fact.Path, Function: fact.Name, Level: types.FunctionQualityLevelElevated, Message: fmt.Sprintf("%s %s quality warning: CTX=%d, LOC=%d (LOC over warn %d)", subject, fact.Name, fact.CTX, fact.LOC, cfg.FunctionQuality.WarnLOC)}, true
	}
	return types.FunctionQualityWarning{}, false
}

// policySubjectLabel returns a human-readable label for the policy subject (function/method).
func policySubjectLabel(fact types.PolicyFact) string {
	if fact.SymbolKind == "method" {
		return "method"
	}
	return "function"
}

// goPolicySymbolKind determines if a Go function is a method or standalone function.
func goPolicySymbolKind(fn *ast.FuncDecl) string {
	if fn.Recv != nil {
		return "method"
	}
	return "function"
}

// hasFileExtension checks if a file path has one of the specified extensions.
func hasFileExtension(path string, extensions []string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, candidate := range extensions {
		if ext == candidate {
			return true
		}
	}
	return false
}

// findFirstAvailableCommand returns the first command from candidates that exists in PATH.
func findFirstAvailableCommand(candidates []string) string {
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate
		}
	}
	return ""
}
