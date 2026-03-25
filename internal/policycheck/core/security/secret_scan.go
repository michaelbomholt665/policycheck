// internal/policycheck/core/security/secret_scan.go
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
			fileViols := ScanGoFileForSecrets(rel, path, string(content), patterns, cfg.SecretLogging)
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

func (v *secretVisitor) checkStringLiteral(lit *ast.BasicLit) {
	val, err := strconv.Unquote(lit.Value)
	if err != nil || val == "" {
		return
	}

	// Contextual Allowlisting
	if len(v.parents) >= 2 {
		parent := v.parents[len(v.parents)-2]

		// 1. Struct Tag
		if field, ok := parent.(*ast.Field); ok && field.Tag == lit {
			return
		}

		// 2. Map Key
		if kv, ok := parent.(*ast.KeyValueExpr); ok && kv.Key == lit {
			return
		}

		// 3. os.Getenv / os.LookupEnv
		if call, ok := parent.(*ast.CallExpr); ok {
			if isOSGetenvCall(call) {
				return
			}
		}

		// 4. Import Path
		if _, ok := parent.(*ast.ImportSpec); ok {
			return
		}
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

	isLogLiteral := false
	for i := len(v.parents) - 1; i >= 0; i-- {
		if call, ok := v.parents[i].(*ast.CallExpr); ok {
			if isLoggingSink(call.Fun, v.cfg) {
				isLogLiteral = true
				break
			}
		}
	}

	if best := evaluateSecretString(val, v.rel, pos.Line, isLogLiteral, v.patterns, v.cfg); best != nil {
		v.viols = append(v.viols, *best)
	}

	// Entropy Gating
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

func checkSuppression(fset *token.FileSet, fileNode *ast.File, pos token.Pos) string {
	line := fset.Position(pos).Line
	for _, cg := range fileNode.Comments {
		for _, comment := range cg.List {
			if fset.Position(comment.Pos()).Line == line {
				if idx := strings.Index(comment.Text, "//nolint:secret"); idx != -1 {
					reasonIdx := strings.Index(comment.Text, "reason:")
					if reasonIdx != -1 {
						return strings.TrimSpace(comment.Text[reasonIdx+7:])
					}
					return "no reason provided"
				}
			}
		}
	}
	return ""
}

func isAllHex(s string) bool {
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if r != '-' {
				return false
			}
		} else {
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return false
			}
		}
	}
	return true
}

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
func ScanGoFileForSecrets(rel, fullPath, content string, patterns []SecretPattern, cfg config.PolicySecretLoggingConfig) []types.Violation {
	fset := token.NewFileSet()
	fileNode, err := parser.ParseFile(fset, fullPath, content, parser.ParseComments)
	if err != nil {
		// Fallback to naive scan if AST fails
		return ScanContentForSecrets(rel, content, patterns, cfg)
	}

	visitor := &secretVisitor{
		fset:     fset,
		fileNode: fileNode,
		rel:      rel,
		patterns: patterns,
		cfg:      cfg,
	}

	ast.Walk(visitor, fileNode)

	return FilterAllowlistedSecretFindings(visitor.viols, cfg)
}

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
		if best := evaluateSecretString(line, rel, i+1, false, patterns, cfg); best != nil {
			allViols = append(allViols, *best)
		}
	}

	return FilterAllowlistedSecretFindings(allViols, cfg)
}

func evaluateSecretString(raw, rel string, lineNum int, isLogLiteral bool, patterns []SecretPattern, cfg config.PolicySecretLoggingConfig) *types.Violation {
	if IsBenignSecretExample(raw, cfg.BenignHints) || IsObviousPlaceholderSecret(raw, cfg.PlaceholderStrings) || IsAllowedLiteral(raw, cfg.CompiledAllowedLiteralPatterns) {
		return nil
	}

	var lineViols []types.Violation

	lineViols = append(lineViols, checkSecretRegexpPatterns(raw, rel, lineNum, isLogLiteral, patterns)...)
	lineViols = append(lineViols, checkSecretKeywords(raw, rel, lineNum, isLogLiteral, cfg.Keywords)...)

	best := PickBestSecretFinding(lineViols)
	if best.RuleID != "" {
		return &best
	}
	return nil
}

func checkSecretRegexpPatterns(raw, rel string, lineNum int, isLogLiteral bool, patterns []SecretPattern) []types.Violation {
	var viols []types.Violation
	for _, p := range patterns {
		if p.Pattern.MatchString(raw) {
			msg := fmt.Sprintf("secret pattern '%s' detected", p.ID)
			if isLogLiteral {
				msg = fmt.Sprintf("secret pattern '%s' detected in log literal", p.ID)
			}
			viols = append(viols, types.Violation{
				RuleID:   p.ID,
				File:     rel,
				Line:     lineNum,
				Message:  msg,
				Severity: p.Severity,
			})
		}
	}
	return viols
}

func checkSecretKeywords(raw, rel string, lineNum int, isLogLiteral bool, keywords []string) []types.Violation {
	var viols []types.Violation

	// 1. Contextual Allowlisting: format strings and error messages
	if strings.Contains(raw, "%s") || strings.Contains(raw, "%d") || strings.Contains(raw, "%v") || strings.Contains(raw, "%q") || strings.Contains(raw, "%w") {
		return viols
	}

	hasAssignment := strings.Contains(raw, "=") || strings.Contains(raw, ":")

	// 2. Sentences: if it has many spaces, it's probably not a secret assignment even if it has a colon/equal.
	if strings.Count(raw, " ") > 2 {
		return viols
	}

	// 3. Short strings without assignments are likely just identifiers or short words.
	if len(raw) < 16 && !hasAssignment {
		return viols
	}

	lowerRaw := strings.ToLower(raw)
	for _, keyword := range keywords {
		lowerKeyword := strings.ToLower(keyword)
		if !strings.Contains(lowerRaw, lowerKeyword) {
			continue
		}

		// Skip if the literal is exactly the keyword or keyword with simple punctuation
		if len(raw) <= len(keyword)+2 {
			continue
		}

		msg := fmt.Sprintf("potential secret keyword '%s' found in log/string literal", keyword)
		if isLogLiteral {
			msg = fmt.Sprintf("potential secret keyword '%s' found in log literal", keyword)
		}
		viols = append(viols, types.Violation{
			RuleID:   "secret-keyword",
			File:     rel,
			Line:     lineNum,
			Message:  msg,
			Severity: "MEDIUM",
		})
	}
	return viols
}
