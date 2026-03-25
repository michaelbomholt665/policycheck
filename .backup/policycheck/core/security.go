// internal/policycheck/core/security.go
// Handles secret logging detection and hardcoded runtime knob warnings.

package core

const ScopeProjectRepo = true


import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/types"
	"policycheck/internal/policycheck/utils"
)

type secretPattern struct {
	id       string
	severity string
	pattern  *regexp.Regexp
}

type secretFinding struct {
	patternID string
	severity  string
}

type secretPatternCatalog struct {
	patterns               []secretPattern
	allowedLiteralPatterns []*regexp.Regexp
	allowlistedPatternIDs  map[string]struct{}
}

type secretScanContext struct {
	fset    *token.FileSet
	cfg     config.PolicyConfig
	catalog secretPatternCatalog
}

type runtimeKnobContext struct {
	cfg            config.PolicyConfig
	assignPattern  *regexp.Regexp
	literalPattern *regexp.Regexp
}

// CheckSecretLoggingPolicies scans source files for potential secrets hardcoded in logging statements.
func CheckSecretLoggingPolicies(root string, cfg config.PolicyConfig) []types.Violation {
	violations := []types.Violation{}
	fset := token.NewFileSet()
	catalog := buildSecretPatternCatalog(cfg)
	scx := secretScanContext{fset: fset, cfg: cfg, catalog: catalog}

	for _, relRoot := range cfg.Paths.SecretScanRoots {
		base := filepath.Join(root, filepath.FromSlash(relRoot))
		_ = filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
			fileViolations := scanSecretLoggingFile(root, base, path, entry, walkErr, scx)
			violations = append(violations, fileViolations...)
			return nil
		})
	}
	return violations
}

// scanSecretLoggingFile scans a single file for secret logging violations.
func scanSecretLoggingFile(root, base, path string, entry fs.DirEntry, walkErr error, scx secretScanContext) []types.Violation {
	if walkErr != nil {
		if os.IsNotExist(walkErr) && path == base {
			return nil
		}
		return []types.Violation{{Path: path, Message: fmt.Sprintf("error walking directory for secret scan: %v", walkErr)}}
	}
	if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
		return nil
	}

	rel := utils.RelOrAbs(root, path)
	if utils.HasPrefix(rel, scx.cfg.SecretLogging.IgnorePathPrefixes) {
		return nil
	}

	fileNode, err := parser.ParseFile(scx.fset, path, nil, parser.ParseComments)
	if err != nil {
		return []types.Violation{{Path: rel, Message: fmt.Sprintf("unable to parse file for secret scan: %v", err)}}
	}
	return collectSecretLoggingViolations(scx.fset, fileNode, rel, scx.cfg, scx.catalog)
}

// collectSecretLoggingViolations inspects a parsed file AST for logging calls containing secrets.
func collectSecretLoggingViolations(fset *token.FileSet, fileNode *ast.File, rel string, cfg config.PolicyConfig, catalog secretPatternCatalog) []types.Violation {
	violations := []types.Violation{}
	ast.Inspect(fileNode, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !isLoggingSink(call.Fun, cfg) {
			return true
		}
		if callAppearsInComment(fset, fileNode, call) {
			return true
		}
		violations = append(violations, collectSecretLiteralViolations(rel, call, cfg, catalog)...)
		return true
	})
	return violations
}

// callAppearsInComment checks if a function call position overlaps with any comment in the file.
func callAppearsInComment(fset *token.FileSet, fileNode *ast.File, call *ast.CallExpr) bool {
	pos := fset.Position(call.Pos())
	for _, cg := range fileNode.Comments {
		for _, comment := range cg.List {
			commentPos := fset.Position(comment.Pos())
			commentEnd := fset.Position(comment.End())
			if pos.Line >= commentPos.Line && pos.Line <= commentEnd.Line {
				return true
			}
		}
	}
	return false
}

// collectSecretLiteralViolations extracts string arguments from a logging call and checks for secrets.
func collectSecretLiteralViolations(rel string, call *ast.CallExpr, cfg config.PolicyConfig, catalog secretPatternCatalog) []types.Violation {
	violations := []types.Violation{}
	for _, arg := range call.Args {
		for _, raw := range extractStringLiterals(arg) {
			finding, ok := evaluateSecretLiteral(raw, cfg, catalog)
			if !ok {
				continue
			}
			violations = append(violations, types.Violation{
				Path:     rel,
				Message:  fmt.Sprintf("potential secret leakage in log literal: %q (%s)", raw, finding.patternID),
				Severity: finding.severity,
			})
		}
	}
	return violations
}

// extractStringLiterals recursively extracts all string literals from an AST expression.
func extractStringLiterals(expr ast.Expr) []string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return extractBasicStringLiteral(e)
	case *ast.BinaryExpr:
		return extractBinaryStringLiterals(e)
	case *ast.ParenExpr:
		return extractStringLiterals(e.X)
	case *ast.CallExpr:
		return extractCallStringLiterals(e)
	default:
		return nil
	}
}

// extractBasicStringLiteral extracts a string literal from a basic literal expression.
func extractBasicStringLiteral(expr *ast.BasicLit) []string {
	if expr.Kind != token.STRING {
		return nil
	}
	val, err := strconv.Unquote(expr.Value)
	if err != nil {
		return nil
	}
	return []string{val}
}

// extractBinaryStringLiterals extracts string literals from string concatenation expressions.
func extractBinaryStringLiterals(expr *ast.BinaryExpr) []string {
	if expr.Op != token.ADD {
		return nil
	}
	literals := extractStringLiterals(expr.X)
	return append(literals, extractStringLiterals(expr.Y)...)
}

// extractCallStringLiterals extracts string literals from formatting call arguments.
func extractCallStringLiterals(expr *ast.CallExpr) []string {
	if !isStringFormattingCall(expr.Fun) || len(expr.Args) == 0 {
		return nil
	}
	literals := make([]string, 0, len(expr.Args))
	for _, arg := range expr.Args {
		literals = append(literals, extractStringLiterals(arg)...)
	}
	return literals
}

// CheckTestFileLocation ensures all test files are in the configured allowed test directories.
func CheckTestFileLocation(root string, cfg config.PolicyConfig) []types.Violation {
	violations := []types.Violation{}
	for _, relBase := range cfg.Paths.TestScanRoots {
		base := filepath.Join(root, filepath.FromSlash(relBase))
		_ = filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
			item := validateTestFileLocation(root, base, path, entry, walkErr, cfg)
			if item != nil {
				violations = append(violations, *item)
			}
			return nil
		})
	}
	return violations
}

// validateTestFileLocation checks if a test file is in an allowed directory.
func validateTestFileLocation(root, base, path string, entry fs.DirEntry, walkErr error, cfg config.PolicyConfig) *types.Violation {
	if walkErr != nil {
		if os.IsNotExist(walkErr) && path == base {
			return nil
		}
		return &types.Violation{Path: path, Message: fmt.Sprintf("error walking directory for test scan: %v", walkErr)}
	}
	if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
		return nil
	}
	rel := utils.RelOrAbs(root, path)
	if utils.HasPrefix(rel, cfg.Paths.AllowedTestPrefixes) {
		return nil
	}
	return &types.Violation{Path: rel, Message: "test files must be located in internal/tests/ to adhere to testing standards"}
}

// CheckCLIOutputFormatterPolicies ensures command handlers use the audience-aware formatter.
func CheckCLIOutputFormatterPolicies(root string, cfg config.PolicyConfig) []types.Violation {
	violations := []types.Violation{}
	fset := token.NewFileSet()

	for _, rel := range cfg.CLIFormatter.RequiredFiles {
		path := filepath.Join(root, filepath.FromSlash(rel))
		fileNode, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			violations = append(violations, types.Violation{Path: rel, Message: fmt.Sprintf("unable to parse file for CLI formatter check: %v", err)})
			continue
		}
		for _, handler := range collectCommandHandlers(fileNode) {
			analysis := analyzeCommandHandler(handler)
			if analysis.HasRawFmtPrint {
				violations = append(violations, types.Violation{Path: rel, Message: fmt.Sprintf("command handler %s uses raw fmt.Print* output; use formatter output", analysis.Name)})
			}
			if !analysis.HasFormatter {
				violations = append(violations, types.Violation{Path: rel, Message: fmt.Sprintf("command handler %s must use GetFormatterFromContext for audience-aware output", analysis.Name)})
			}
		}
	}
	return violations
}

// collectCommandHandlers extracts command handler functions from a parsed Go file.
func collectCommandHandlers(fileNode *ast.File) []*ast.FuncDecl {
	handlers := []*ast.FuncDecl{}
	for _, decl := range fileNode.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || !isCommandHandlerFunc(fn) {
			continue
		}
		handlers = append(handlers, fn)
	}
	return handlers
}

// isCommandHandlerFunc determines if a function declaration is a CLI command handler.
func isCommandHandlerFunc(fn *ast.FuncDecl) bool {
	if fn == nil || fn.Recv != nil || fn.Name == nil || fn.Body == nil {
		return false
	}
	return fn.Name.Name == "main" || strings.HasPrefix(fn.Name.Name, "Run")
}

// analyzeCommandHandler inspects a command handler function for formatter usage.
func analyzeCommandHandler(fn *ast.FuncDecl) types.CommandHandlerAnalysis {
	analysis := types.CommandHandlerAnalysis{Name: fn.Name.Name}
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if isFormatterCall(call.Fun) {
			analysis.HasFormatter = true
		}
		if isFmtPrintCall(call.Fun) {
			analysis.HasRawFmtPrint = true
		}
		return true
	})
	return analysis
}

// isFmtPrintCall checks if an expression is a raw fmt.Print* call.
func isFmtPrintCall(fun ast.Expr) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "fmt" {
		return false
	}
	switch sel.Sel.Name {
	case "Print", "Printf", "Println":
		return true
	default:
		return false
	}
}

// isFormatterCall checks if an expression calls the audience-aware formatter.
func isFormatterCall(fun ast.Expr) bool {
	switch e := fun.(type) {
	case *ast.Ident:
		return e.Name == "GetFormatterFromContext"
	case *ast.SelectorExpr:
		if e.Sel.Name == "GetFormatterFromContext" {
			return true
		}
		if xIdent, ok := e.X.(*ast.Ident); ok {
			return strings.ToLower(xIdent.Name) == "formatter"
		}
	}
	return false
}

// CheckHardcodedRuntimeKnobWarnings scans for hardcoded values that should be in configuration.
func CheckHardcodedRuntimeKnobWarnings(root string, cfg config.PolicyConfig) []types.Violation {
	warnings := []types.Violation{}
	assignPattern, literalPattern, err := compileRuntimeKnobPatterns(cfg.HardcodedRuntimeKnob.Identifiers)
	if err != nil {
		warnings = append(warnings, types.Violation{Path: "policy-gate.toml", Message: fmt.Sprintf("unable to prepare runtime knob detection patterns: %v", err)})
		return warnings
	}

	rcx := runtimeKnobContext{cfg: cfg, assignPattern: assignPattern, literalPattern: literalPattern}
	for _, relBase := range cfg.Paths.HardcodedRuntimeKnobScanRoots {
		base := filepath.Join(root, filepath.FromSlash(relBase))
		_ = filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
			warnings = append(warnings, scanRuntimeKnobFile(root, base, path, entry, walkErr, rcx)...)
			return nil
		})
	}
	return warnings
}

// scanRuntimeKnobFile scans a single file for hardcoded runtime knob violations.
func scanRuntimeKnobFile(root, base, path string, entry fs.DirEntry, walkErr error, rcx runtimeKnobContext) []types.Violation {
	if walkErr != nil {
		if os.IsNotExist(walkErr) && path == base {
			return nil
		}
		return []types.Violation{{Path: path, Message: fmt.Sprintf("error walking directory for runtime knob scan: %v", walkErr)}}
	}
	if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
		return nil
	}
	rel := utils.RelOrAbs(root, path)
	if utils.HasPrefix(rel, rcx.cfg.Paths.HardcodedRuntimeKnobIgnorePath) {
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return []types.Violation{{Path: rel, Message: fmt.Sprintf("unable to read file for runtime knob check: %v", err)}}
	}
	return collectRuntimeKnobWarnings(rel, content, rcx.assignPattern, rcx.literalPattern)
}

// collectRuntimeKnobWarnings checks file content for hardcoded runtime configuration values.
func collectRuntimeKnobWarnings(rel string, content []byte, assignPattern, literalPattern *regexp.Regexp) []types.Violation {
	warnings := []types.Violation{}
	for idx, line := range strings.Split(string(content), "\n") {
		if !assignPattern.MatchString(line) && !literalPattern.MatchString(line) {
			continue
		}
		warnings = append(warnings, types.Violation{
			Path:    fmt.Sprintf("%s:%d", rel, idx+1),
			Message: "possible hardcoded runtime knob; prefer TOML-driven config",
		})
	}
	return warnings
}

// compileRuntimeKnobPatterns prepares regular expressions for detecting hardcoded runtime knobs.
func compileRuntimeKnobPatterns(identifiers []string) (*regexp.Regexp, *regexp.Regexp, error) {
	pattern := strings.Join(quoteMetaAll(identifiers), "|")
	assignPattern, err := regexp.Compile(fmt.Sprintf(`^\s*(%s)\s*(?:=|:=)\s*([0-9]+|true|false|"[^"]*"|'[^']*')`, pattern))
	if err != nil {
		return nil, nil, fmt.Errorf("compile runtime knob assignment pattern: %w", err)
	}
	literalPattern, err := regexp.Compile(fmt.Sprintf(`\b(%s)\s*:\s*([0-9]+|true|false|"[^"]*"|'[^']*')`, pattern))
	if err != nil {
		return nil, nil, fmt.Errorf("compile runtime knob literal pattern: %w", err)
	}
	return assignPattern, literalPattern, nil
}

// quoteMetaAll escapes all regex metacharacters in the given string values.
func quoteMetaAll(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, regexp.QuoteMeta(value))
	}
	return result
}

// buildSecretPatternCatalog constructs the secret detection pattern catalog with overrides applied.
func buildSecretPatternCatalog(cfg config.PolicyConfig) secretPatternCatalog {
	patterns := make([]secretPattern, 0, len(builtInSecretPatterns))
	for _, item := range builtInSecretPatterns {
		severity, enabled := resolveSecretPatternSeverity(item, cfg.SecretLogging.Overrides)
		if !enabled {
			continue
		}
		patterns = append(patterns, secretPattern{id: item.id, severity: severity, pattern: item.pattern})
	}
	return secretPatternCatalog{
		patterns:               patterns,
		allowedLiteralPatterns: cfg.SecretLogging.CompiledAllowedLiteralPatterns,
		allowlistedPatternIDs:  buildAllowlistedPatternIDs(cfg.SecretLogging.Allowlist.PatternIDs),
	}
}

// evaluateSecretLiteral determines if a string literal contains a potential secret.
func evaluateSecretLiteral(raw string, cfg config.PolicyConfig, catalog secretPatternCatalog) (secretFinding, bool) {
	lower := strings.ToLower(raw)
	if shouldIgnoreSecretLiteral(raw, lower, catalog) {
		return secretFinding{}, false
	}

	allMatches := collectSecretMatches(raw, catalog.patterns)
	matches := filterAllowlistedSecretFindings(allMatches, catalog.allowlistedPatternIDs)
	if finding, ok := pickBestSecretFinding(matches); ok {
		return finding, true
	}
	if len(allMatches) > 0 {
		return secretFinding{}, false
	}
	if hasSecretKeyword(lower, cfg.SecretLogging.Keywords) {
		return secretFinding{patternID: "keyword_match", severity: secretSeverityLow}, true
	}
	return secretFinding{}, false
}

// resolveSecretPatternSeverity applies config overrides to a built-in secret pattern.
func resolveSecretPatternSeverity(item secretPattern, overrides map[string]string) (string, bool) {
	severity := item.severity
	override, ok := overrides[item.id]
	if !ok {
		return severity, true
	}
	switch normalized := strings.ToUpper(strings.TrimSpace(override)); normalized {
	case "DISABLED", "OFF":
		return "", false
	case secretSeverityLow, secretSeverityMedium, secretSeverityHigh, secretSeverityCritical:
		return normalized, true
	default:
		return severity, true
	}
}

// buildAllowlistedPatternIDs constructs a set of allowlisted secret pattern identifiers.
func buildAllowlistedPatternIDs(patternIDs []string) map[string]struct{} {
	allowlistedPatternIDs := make(map[string]struct{}, len(patternIDs))
	for _, id := range patternIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			allowlistedPatternIDs[trimmed] = struct{}{}
		}
	}
	return allowlistedPatternIDs
}

// shouldIgnoreSecretLiteral reports whether a literal should be excluded before pattern matching.
func shouldIgnoreSecretLiteral(raw, lower string, catalog secretPatternCatalog) bool {
	if isAllowedSecretLogText(lower) || matchesSecretAllowPattern(raw, catalog.allowedLiteralPatterns) {
		return true
	}
	return isBenignSecretExample(lower) || isObviousPlaceholderSecret(raw)
}

// collectSecretMatches returns all catalog patterns that match the raw literal.
func collectSecretMatches(raw string, patterns []secretPattern) []secretFinding {
	matches := make([]secretFinding, 0, 4)
	for _, item := range patterns {
		if item.pattern.MatchString(raw) {
			matches = append(matches, secretFinding{patternID: item.id, severity: item.severity})
		}
	}
	return matches
}

// pickBestSecretFinding selects the highest-priority secret finding from a list of matches.
func pickBestSecretFinding(matches []secretFinding) (secretFinding, bool) {
	if len(matches) == 0 {
		return secretFinding{}, false
	}
	filtered := matches
	if hasSpecificSecretFinding(matches) {
		filtered = make([]secretFinding, 0, len(matches))
		for _, match := range matches {
			if !isGenericSecretPattern(match.patternID) {
				filtered = append(filtered, match)
			}
		}
	}
	best := filtered[0]
	bestRank := secretSeverityRank(best.severity)
	for _, match := range filtered[1:] {
		if matchRank := secretSeverityRank(match.severity); matchRank > bestRank {
			best = match
			bestRank = matchRank
		}
	}
	return best, true
}

// filterAllowlistedSecretFindings removes findings that match allowlisted pattern IDs.
func filterAllowlistedSecretFindings(matches []secretFinding, allowlistedPatternIDs map[string]struct{}) []secretFinding {
	filtered := matches
	if hasSpecificSecretFinding(matches) {
		filtered = make([]secretFinding, 0, len(matches))
		for _, match := range matches {
			if !isGenericSecretPattern(match.patternID) {
				filtered = append(filtered, match)
			}
		}
	}
	result := make([]secretFinding, 0, len(filtered))
	for _, match := range filtered {
		if _, ok := allowlistedPatternIDs[match.patternID]; !ok {
			result = append(result, match)
		}
	}
	return result
}

// hasSpecificSecretFinding returns true if any finding is a specific (not generic) secret pattern.
func hasSpecificSecretFinding(matches []secretFinding) bool {
	for _, match := range matches {
		if !isGenericSecretPattern(match.patternID) {
			return true
		}
	}
	return false
}

// isGenericSecretPattern returns true if the pattern ID represents a generic secret type.
func isGenericSecretPattern(patternID string) bool {
	_, ok := genericSecretPatternIDs[patternID]
	return ok
}

// secretSeverityRank returns a numeric priority for secret severity levels.
func secretSeverityRank(severity string) int {
	switch severity {
	case secretSeverityCritical:
		return 4
	case secretSeverityHigh:
		return 3
	case secretSeverityMedium:
		return 2
	case secretSeverityLow:
		return 1
	default:
		return 0
	}
}

// isKnownLoggerIdentifier checks if a name matches known logging library identifiers.
func isKnownLoggerIdentifier(name string, cfg config.PolicyConfig) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if _, ok := knownLoggerIdentifiers[lower]; ok {
		return true
	}
	for _, ident := range cfg.SecretLogging.LoggerIdentifiers {
		if strings.ToLower(ident) == lower {
			return true
		}
	}
	return false
}

// isBenignSecretExample returns true if the text appears to be a benign example or placeholder.
func isBenignSecretExample(lower string) bool {
	for _, hint := range []string{"example", "sample", "placeholder", "dummy", "fake", "fixture", "redacted", "masked", "your-", "your_", "test-", "test_", "my-", "my_"} {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

// isObviousPlaceholderSecret returns true if the text is an obvious placeholder value.
func isObviousPlaceholderSecret(raw string) bool {
	lower := strings.ToLower(raw)
	for _, placeholder := range []string{"<token>", "<password>", "<secret>", "<api-key>", "your_token_here", "replace_me", "changeme", "change_me"} {
		if strings.Contains(lower, placeholder) {
			return true
		}
	}
	for _, pattern := range obviousPlaceholderSecretPatterns {
		if pattern.MatchString(raw) {
			return true
		}
	}
	return false
}

// isLoggingSink determines if an expression is a call to a logging library method.
func isLoggingSink(fun ast.Expr, cfg config.PolicyConfig) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	receiver := strings.ToLower(selectorBaseName(sel.X))
	if receiver == "" || !isKnownLoggerIdentifier(receiver, cfg) {
		return false
	}
	switch strings.ToLower(sel.Sel.Name) {
	case "warn", "warning", "error", "info", "infooneline", "printf", "print", "println", "fatal", "fatalf", "panic", "panicf":
		return true
	default:
		return false
	}
}

// hasSecretKeyword returns true if the raw text contains any configured secret keywords.
func hasSecretKeyword(raw string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(raw, keyword) {
			return true
		}
	}
	return false
}

// matchesSecretAllowPattern checks if raw text matches any allowed secret patterns.
func matchesSecretAllowPattern(raw string, patterns []*regexp.Regexp) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(raw) {
			return true
		}
	}
	return false
}

// isAllowedSecretLogText returns true if the text indicates intentional redaction or masking.
func isAllowedSecretLogText(lower string) bool {
	return strings.Contains(lower, "redact") || strings.Contains(lower, "mask")
}

// isStringFormattingCall checks if an expression is a fmt.Sprintf or fmt.Errorf call.
func isStringFormattingCall(fun ast.Expr) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok || strings.ToLower(pkgIdent.Name) != "fmt" {
		return false
	}
	switch sel.Sel.Name {
	case "Sprintf", "Errorf":
		return true
	default:
		return false
	}
}

// selectorBaseName extracts the base identifier from a selector expression.
func selectorBaseName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return value.Sel.Name
	default:
		return ""
	}
}
