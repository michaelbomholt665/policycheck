// internal/policycheck/core/security/secret_scan.go
// Package security/secret_scan implements the scanning logic for identifying secrets.
// It performs entropy analysis and pattern matching across the repository.
package security

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"math"
	"strconv"
	"strings"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/host"
	"policycheck/internal/policycheck/types"
	"policycheck/internal/policycheck/utils"
)

// CheckSecretLoggingPolicies scans the repository for potentially leaked secrets in logs.
func CheckSecretLoggingPolicies(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation {
	walk, err := host.ResolveWalkProvider()
	if err != nil {
		return []types.Violation{{
			RuleID:   "secret-logging",
			Message:  fmt.Sprintf("resolve walk provider: %v", err),
			Severity: "error",
		}}
	}

	var viols []types.Violation
	patterns := BuiltInPatterns()

	err = walk.WalkDirectoryTree(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}

		rel, _ := host.RelOrAbs(root, path)

		// Check if file is in scan roots
		if host.HasPrefix(rel, cfg.SecretLogging.IgnorePathPrefixes) {
			return nil
		}

		if !host.HasPrefix(rel, cfg.Paths.SecretScanRoots) {
			return nil
		}

		content, err := host.ReadFile(path)
		if err != nil {
			return nil
		}

		if strings.HasSuffix(path, ".go") {
			fileViols := ScanGoFileForSecrets(secretScanInput{
				rel:      rel,
				fullPath: path,
				content:  string(content),
			}, patterns, cfg.SecretLogging)
			viols = append(viols, fileViols...)
		} else {
			fileViols := ScanContentForSecrets(rel, string(content), patterns, cfg.SecretLogging)
			viols = append(viols, fileViols...)
		}

		return nil
	})
	if err != nil {
		viols = append(viols, types.Violation{
			RuleID:   "secret-logging",
			Message:  fmt.Sprintf("walk directory: %v", err),
			Severity: "error",
		})
	}

	return viols
}

type secretVisitor struct {
	fset     *token.FileSet
	fileNode *ast.File
	rel      string
	patterns []SecretPattern
	cfg      config.PolicySecretLoggingConfig
	viols    []types.Violation
	parents  []ast.Node
}

type secretContext struct {
	rel          string
	lineNum      int
	isLogLiteral bool
	patterns     []SecretPattern
	cfg          config.PolicySecretLoggingConfig
}

type secretScanInput struct {
	rel      string
	fullPath string
	content  string
}

// Visit implements the ast.Visitor interface to traverse the Go AST for strings.
func (v *secretVisitor) Visit(n ast.Node) ast.Visitor {
	if n == nil {
		v.parents = v.parents[:len(v.parents)-1]
		return nil
	}
	v.parents = append(v.parents, n)

	if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		v.checkStringLiteral(lit)
	}

	return v
}

// checkStringLiteral evaluates a single string literal for secrets and entropy.
func (v *secretVisitor) checkStringLiteral(lit *ast.BasicLit) {
	val, err := strconv.Unquote(lit.Value)
	if err != nil || val == "" {
		return
	}

	if v.checkContextualAllowlist(lit) {
		return
	}

	pos := v.fset.Position(lit.Pos())

	// Inline Suppression
	reason := checkSuppression(v.fset, v.fileNode, lit.Pos())
	if reason != "" {
		v.viols = append(v.viols, types.Violation{
			RuleID:   "secret-suppressed",
			File:     v.rel,
			Line:     pos.Line,
			Message:  "Suppressed secret: " + reason,
			Severity: "suppressed",
		})
		return
	}

	isLogLiteral := v.isContextLoggingSink()
	if best := evaluateSecretString(val, secretContext{
		rel:          v.rel,
		lineNum:      pos.Line,
		isLogLiteral: isLogLiteral,
		patterns:     v.patterns,
		cfg:          v.cfg,
	}); best != nil {
		v.viols = append(v.viols, *best)
	}

	v.checkEntropy(val, pos)
}

// checkContextualAllowlist returns true if the literal is in a safe context (e.g. map key).
func (v *secretVisitor) checkContextualAllowlist(lit *ast.BasicLit) bool {
	if len(v.parents) < 2 {
		return false
	}
	parent := v.parents[len(v.parents)-2]

	// 1. Struct Tag
	if field, ok := parent.(*ast.Field); ok && field.Tag == lit {
		return true
	}

	// 2. Map Key
	if kv, ok := parent.(*ast.KeyValueExpr); ok && kv.Key == lit {
		return true
	}

	// 3. os.Getenv / os.LookupEnv
	if call, ok := parent.(*ast.CallExpr); ok {
		if isOSGetenvCall(call) {
			return true
		}
	}

	// 4. Import Path
	if _, ok := parent.(*ast.ImportSpec); ok {
		return true
	}
	return false
}

// isContextLoggingSink returns true if the current node is within a logging-system call.
func (v *secretVisitor) isContextLoggingSink() bool {
	for i := len(v.parents) - 1; i >= 0; i-- {
		if call, ok := v.parents[i].(*ast.CallExpr); ok {
			if isLoggingSink(call.Fun, v.cfg) {
				return true
			}
		}
	}
	return false
}

// checkEntropy calculates and reports high information density strings.
func (v *secretVisitor) checkEntropy(val string, pos token.Position) {
	if len(val) > 32 && !strings.Contains(val, " ") && !isAllHex(val) && !isUUID(val) {
		ent := calculateShannonEntropy(val)
		if ent > 4.5 {
			v.viols = append(v.viols, types.Violation{
				RuleID:   "high-entropy-string",
				File:     v.rel,
				Line:     pos.Line,
				Message:  fmt.Sprintf("high entropy string detected (entropy: %.2f)", ent),
				Severity: "error",
			})
		}
	}
}

// isOSGetenvCall returns true if the call expression is for os.Getenv or os.LookupEnv.
func isOSGetenvCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return x.Name == "os" && (sel.Sel.Name == "Getenv" || sel.Sel.Name == "LookupEnv")
}

// checkSuppression looks for inline //nolint:secret comments for a given AST position.
func checkSuppression(fset *token.FileSet, fileNode *ast.File, pos token.Pos) string {
	line := fset.Position(pos).Line
	for _, cg := range fileNode.Comments {
		if reason, ok := checkCommentGroupForSuppression(fset, cg, line); ok {
			return reason
		}
	}
	return ""
}

// checkCommentGroupForSuppression checks a comment group's lines for secret suppression markers.
func checkCommentGroupForSuppression(fset *token.FileSet, cg *ast.CommentGroup, line int) (string, bool) {
	for _, comment := range cg.List {
		if fset.Position(comment.Pos()).Line != line {
			continue
		}
		if idx := strings.Index(comment.Text, "//nolint:secret"); idx != -1 {
			reasonIdx := strings.Index(comment.Text, "reason:")
			if reasonIdx != -1 {
				return strings.TrimSpace(comment.Text[reasonIdx+7:]), true
			}
			return "no reason provided", true
		}
	}
	return "", false
}

// isAllHex returns true if the string contains only hexadecimal digits.
func isAllHex(s string) bool {
	for _, r := range s {
		if !isHexDigit(r) {
			return false
		}
	}
	return true
}

// isUUID returns true if the string matches the standard UUID format.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if r != '-' {
				return false
			}
		} else if !isHexDigit(r) {
			return false
		}
	}
	return true
}

// isHexDigit returns true if the rune is a valid hexadecimal character.
func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

// calculateShannonEntropy measures the information density of a string.
func calculateShannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	counts := make(map[rune]float64)
	for _, r := range s {
		counts[r]++
	}
	var entropy float64
	total := float64(len(s))
	for _, count := range counts {
		prob := count / total
		entropy -= prob * math.Log2(prob)
	}
	return entropy
}

// ScanGoFileForSecrets uses AST parsing to find string literals and evaluate them for secrets.
func ScanGoFileForSecrets(input secretScanInput, patterns []SecretPattern, cfg config.PolicySecretLoggingConfig) []types.Violation {
	fset := token.NewFileSet()
	fileNode, err := parser.ParseFile(fset, input.fullPath, input.content, parser.ParseComments)
	if err != nil {
		// Fallback to naive scan if AST fails
		return ScanContentForSecrets(input.rel, input.content, patterns, cfg)
	}

	visitor := &secretVisitor{
		fset:     fset,
		fileNode: fileNode,
		rel:      input.rel,
		patterns: patterns,
		cfg:      cfg,
	}

	ast.Walk(visitor, fileNode)

	return FilterAllowlistedSecretFindings(visitor.viols, cfg)
}

// isLoggingSink returns true if the function call is identified as a logging operation.
func isLoggingSink(fun ast.Expr, cfg config.PolicySecretLoggingConfig) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	receiver := strings.ToLower(utils.SelectorBaseName(sel.X))
	if receiver == "" {
		return false
	}
	if !isKnownLoggerIdentifier(receiver, cfg) {
		return false
	}

	name := strings.ToLower(sel.Sel.Name)
	switch name {
	case "warn", "warning", "error", "info", "infooneline", "printf", "print", "println", "fatal", "fatalf", "panic", "panicf":
		return true
	default:
		return false
	}
}

// isKnownLoggerIdentifier returns true if the receiver name is a common or configured logger.
func isKnownLoggerIdentifier(name string, cfg config.PolicySecretLoggingConfig) bool {
	// Built-in defaults
	defaults := map[string]struct{}{
		"log":     {},
		"logger":  {},
		"zap":     {},
		"zerolog": {},
		"fmt":     {},
		"os":      {},
		"ctxlog":  {},
		"l":       {},
	}
	if _, ok := defaults[name]; ok {
		return true
	}
	for _, id := range cfg.LoggerIdentifiers {
		if strings.ToLower(id) == name {
			return true
		}
	}
	return false
}

// extractStringLiterals recursively pulls string literal values from an AST expression.
func extractStringLiterals(expr ast.Expr) []string {
	var literals []string
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			if val, err := strconv.Unquote(e.Value); err == nil {
				literals = append(literals, val)
			}
		}
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			literals = append(literals, extractStringLiterals(e.X)...)
			literals = append(literals, extractStringLiterals(e.Y)...)
		}
	case *ast.ParenExpr:
		literals = append(literals, extractStringLiterals(e.X)...)
	case *ast.CallExpr:
		if !isStringFormattingCall(e.Fun) || len(e.Args) == 0 {
			return literals
		}
		for _, arg := range e.Args {
			literals = append(literals, extractStringLiterals(arg)...)
		}
	}
	return literals
}

// isStringFormattingCall returns true if the function is a fmt formatting call.
func isStringFormattingCall(fun ast.Expr) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	if strings.ToLower(pkgIdent.Name) != "fmt" {
		return false
	}

	switch sel.Sel.Name {
	case "Sprintf", "Errorf":
		return true
	default:
		return false
	}
}

// ScanContentForSecrets evaluates a single file's content for secret patterns.
func ScanContentForSecrets(rel, content string, patterns []SecretPattern, cfg config.PolicySecretLoggingConfig) []types.Violation {
	var allViols []types.Violation

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if best := evaluateSecretString(line, secretContext{
			rel:      rel,
			lineNum:  i + 1,
			patterns: patterns,
			cfg:      cfg,
		}); best != nil {
			allViols = append(allViols, *best)
		}
	}

	return FilterAllowlistedSecretFindings(allViols, cfg)
}

// evaluateSecretString runs all regex and keyword checks against a raw string.
func evaluateSecretString(raw string, ctx secretContext) *types.Violation {
	if IsBenignSecretExample(raw, ctx.cfg.BenignHints) || IsObviousPlaceholderSecret(raw, ctx.cfg.PlaceholderStrings) || IsAllowedLiteral(raw, ctx.cfg.CompiledAllowedLiteralPatterns) {
		return nil
	}

	var lineViols []types.Violation

	lineViols = append(lineViols, checkSecretRegexpPatterns(raw, ctx)...)
	lineViols = append(lineViols, checkSecretKeywords(raw, ctx)...)

	best := PickBestSecretFinding(lineViols)
	if best.RuleID != "" {
		return &best
	}
	return nil
}

// checkSecretRegexpPatterns matches a string against the set of compiled secret regexes.
func checkSecretRegexpPatterns(raw string, ctx secretContext) []types.Violation {
	var viols []types.Violation
	for _, p := range ctx.patterns {
		if p.Pattern.MatchString(raw) {
			msg := fmt.Sprintf("secret pattern '%s' detected", p.ID)
			if ctx.isLogLiteral {
				msg = fmt.Sprintf("secret pattern '%s' detected in log literal", p.ID)
			}
			viols = append(viols, types.Violation{
				RuleID:   p.ID,
				File:     ctx.rel,
				Line:     ctx.lineNum,
				Message:  msg,
				Severity: p.Severity,
			})
		}
	}
	return viols
}

// checkSecretKeywords matches a string against a list of sensitive identity keywords.
func checkSecretKeywords(raw string, ctx secretContext) []types.Violation {
	var viols []types.Violation

	if shouldSkipKeywordScan(raw) {
		return viols
	}

	lowerRaw := strings.ToLower(raw)
	for _, keyword := range ctx.cfg.Keywords {
		lowerKeyword := strings.ToLower(keyword)
		if !strings.Contains(lowerRaw, lowerKeyword) {
			continue
		}

		if isTriviallyShortKeywordLiteral(raw, keyword) {
			continue
		}

		msg := buildSecretKeywordMessage(keyword, ctx.isLogLiteral)
		viols = append(viols, types.Violation{
			RuleID:   "secret-keyword",
			File:     ctx.rel,
			Line:     ctx.lineNum,
			Message:  msg,
			Severity: "MEDIUM",
		})
	}
	return viols
}

// shouldSkipKeywordScan applies heuristics to filter out obviously non-sensitive literals.
func shouldSkipKeywordScan(raw string) bool {
	if containsFormattingDirective(raw) {
		return true
	}

	if strings.Count(raw, " ") > 2 {
		return true
	}

	hasAssignment := strings.Contains(raw, "=") || strings.Contains(raw, ":")
	return len(raw) < 16 && !hasAssignment
}

// containsFormattingDirective returns true if the string has Go/C-style format markers.
func containsFormattingDirective(raw string) bool {
	formatTokens := []string{"%s", "%d", "%v", "%q", "%w"}
	for _, token := range formatTokens {
		if strings.Contains(raw, token) {
			return true
		}
	}
	return false
}

// isTriviallyShortKeywordLiteral returns true if the literal is barely longer than the keyword itself.
func isTriviallyShortKeywordLiteral(raw, keyword string) bool {
	return len(raw) <= len(keyword)+2
}

// buildSecretKeywordMessage constructs a severity-specific violation message.
func buildSecretKeywordMessage(keyword string, isLogLiteral bool) string {
	if isLogLiteral {
		return fmt.Sprintf("potential secret keyword '%s' found in log literal", keyword)
	}

	return fmt.Sprintf("potential secret keyword '%s' found in log/string literal", keyword)
}
