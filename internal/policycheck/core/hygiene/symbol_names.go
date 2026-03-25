// internal/policycheck/core/hygiene/symbol_names.go
package hygiene

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"unicode"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/host"
	"policycheck/internal/policycheck/types"
	"policycheck/internal/policycheck/utils"
)

// knownAcronyms lists mixed-case forms that violate Go acronym casing rules.
// Each entry is the incorrect form; the correct form is the all-caps equivalent.
var knownAcronyms = []string{
	"Http", "Url", "Id", "Json", "Xml", "Dsn", "Api",
}

// crossBackendDirs are package path segments that indicate a symbol is part of
// a cross-backend surface and therefore requires the 3-token minimum.
var crossBackendDirs = []string{
	"ports", "types", "contracts",
}

// CheckSymbolNames validates that exported symbols meet naming conventions
// across all configured scan roots.
func CheckSymbolNames(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation {
	walk, err := host.ResolveWalkProvider()
	if err != nil {
		return nil
	}

	scanRoots := resolveScanRoots(cfg)
	var viols []types.Violation

	for _, scanRoot := range scanRoots {
		absRoot := filepath.Join(root, scanRoot)
		walk.WalkDirectoryTree(absRoot, func(path string, d fs.DirEntry, err error) error {
			return collectSymbolViolations(root, path, d, err, cfg, &viols)
		})
	}

	return viols
}

// resolveScanRoots returns the effective list of roots to scan, falling back
// to ProductionRoots when no explicit HygieneScanRoots are configured.
func resolveScanRoots(cfg config.PolicyConfig) []string {
	if len(cfg.Hygiene.ScanRoots) > 0 {
		return cfg.Hygiene.ScanRoots
	}
	return cfg.Paths.ProductionRoots
}

// collectSymbolViolations is the per-entry callback for the directory walk.
// It filters non-Go files and excluded paths before delegating to file-level checks.
func collectSymbolViolations(
	root, path string,
	d fs.DirEntry,
	err error,
	cfg config.PolicyConfig,
	viols *[]types.Violation,
) error {
	if err != nil || d.IsDir() || filepath.Ext(path) != ".go" {
		return nil
	}
	rel := utils.ToSlashRel(root, path)
	if isExcluded(rel, cfg.Hygiene.ExcludePrefixes) {
		return nil
	}
	*viols = append(*viols, checkFileSymbolNames(root, path, cfg)...)
	return nil
}

// isExcluded returns true when rel matches any of the configured exclude prefixes
// or hardcoded internal tool paths.
func isExcluded(rel string, prefixes []string) bool {
	// Hardcoded exclusions for the tool itself
	if strings.HasPrefix(rel, "cmd/policycheck/") || strings.HasPrefix(rel, "internal/policycheck/") {
		return true
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(rel, prefix) {
			return true
		}
	}
	return false
}

// checkFileSymbolNames parses a single Go file and validates all exported symbols.
func checkFileSymbolNames(root, path string, cfg config.PolicyConfig) []types.Violation {
	content, err := host.ReadFile(path)
	if err != nil {
		return nil
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return nil
	}

	rel := utils.ToSlashRel(root, path)
	minTokens := resolveMinTokens(rel, cfg.Hygiene.MinNameTokens, cfg.Hygiene.CrossBackendMinNameTokens)

	var viols []types.Violation
	ast.Inspect(f, func(n ast.Node) bool {
		viols = append(viols, inspectNode(n, rel, fset, minTokens, cfg.Hygiene.ExemptFunctionNames)...)
		return true
	})
	return viols
}

// resolveMinTokens returns the effective minimum token count for a file.
// Cross-backend surface files require 3 tokens; all others require the configured
// minimum, defaulting to 2 when not set.
func resolveMinTokens(rel string, configured int, crossBackendConfigured int) int {
	for _, dir := range crossBackendDirs {
		if strings.Contains(rel, dir) {
			if crossBackendConfigured > 0 {
				return crossBackendConfigured
			}
			return 3
		}
	}
	if configured > 0 {
		return configured
	}
	return 2
}

// inspectNode extracts naming violations from a single AST node.
func inspectNode(n ast.Node, rel string, fset *token.FileSet, minTokens int, exemptNames []string) []types.Violation {
	switch decl := n.(type) {
	case *ast.FuncDecl:
		return checkFuncDecl(decl, rel, fset, minTokens, exemptNames)
	case *ast.TypeSpec:
		return checkTypeSpec(decl, rel, fset, minTokens)
	case *ast.ValueSpec:
		return checkValueSpec(decl, rel, fset, minTokens)
	}
	return nil
}

// checkFuncDecl validates an exported function declaration.
func checkFuncDecl(decl *ast.FuncDecl, rel string, fset *token.FileSet, minTokens int, exemptNames []string) []types.Violation {
	if decl.Name == nil || !decl.Name.IsExported() {
		return nil
	}
	name := decl.Name.Name
	for _, exempt := range exemptNames {
		if name == exempt {
			return nil
		}
	}
	return validateSymbol(rel, name, minTokens, fset.Position(decl.Pos()).Line)
}

// checkTypeSpec validates an exported type declaration.
func checkTypeSpec(decl *ast.TypeSpec, rel string, fset *token.FileSet, minTokens int) []types.Violation {
	if decl.Name == nil || !decl.Name.IsExported() {
		return nil
	}
	return validateSymbol(rel, decl.Name.Name, minTokens, fset.Position(decl.Pos()).Line)
}

// checkValueSpec validates exported variable and constant declarations.
func checkValueSpec(decl *ast.ValueSpec, rel string, fset *token.FileSet, minTokens int) []types.Violation {
	var viols []types.Violation
	for _, name := range decl.Names {
		if !name.IsExported() {
			continue
		}
		viols = append(viols, validateSymbol(rel, name.Name, minTokens, fset.Position(name.Pos()).Line)...)
	}
	return viols
}

// validateSymbol runs all naming checks against a single exported symbol name.
// Acronym casing is checked first; token count is checked second.
func validateSymbol(rel, name string, minTokens, line int) []types.Violation {
	if v := checkAcronymCasing(rel, name, line); v != nil {
		return []types.Violation{*v}
	}
	if v := checkTokenCount(rel, name, minTokens, line); v != nil {
		return []types.Violation{*v}
	}
	return nil
}

// checkAcronymCasing returns a violation when name contains a known acronym in
// mixed-case form (e.g. Http instead of HTTP).
func checkAcronymCasing(rel, name string, line int) *types.Violation {
	for _, bad := range knownAcronyms {
		if !containsWholeToken(name, bad) {
			continue
		}
		correct := strings.ToUpper(bad)
		return &types.Violation{
			RuleID:   "hygiene.symbol_names",
			File:     rel,
			Function: name,
			Line:     line,
			Message:  fmt.Sprintf("exported symbol %q uses incorrect acronym casing: use %s, not %s", name, correct, bad),
			Severity: "error",
		}
	}
	return nil
}

// containsWholeToken returns true when sub appears as a complete CamelCase token
// within name — i.e. it is preceded by an uppercase letter or start-of-string,
// and followed by an uppercase letter or end-of-string.
// This prevents "Http" matching inside "HttpsOnly" while correctly catching "HttpClient".
func containsWholeToken(name, sub string) bool {
	idx := strings.Index(name, sub)
	if idx < 0 {
		return false
	}
	end := idx + len(sub)
	if end < len(name) && unicode.IsLower(rune(name[end])) {
		return false
	}
	return true
}

// checkTokenCount returns a violation when name contains fewer tokens than minTokens.
func checkTokenCount(rel, name string, minTokens, line int) *types.Violation {
	count := CountTokens(name)
	if count >= minTokens {
		return nil
	}
	return &types.Violation{
		RuleID:   "hygiene.symbol_names",
		File:     rel,
		Function: name,
		Line:     line,
		Message:  fmt.Sprintf("exported symbol %q has %d token(s), minimum is %d", name, count, minTokens),
		Severity: "error",
	}
}

// CountTokens counts the number of semantic tokens in a Go identifier.
// It handles CamelCase, acronym runs (e.g. HTTP), and underscore-separated names.
func CountTokens(name string) int {
	if name == "main" {
		return 2
	}
	if name == "" {
		return 0
	}

	runes := []rune(name)
	tokens := 0
	i := 0

	for i < len(runes) {
		if runes[i] == '_' {
			i++
			continue
		}
		tokens++
		i = skipToken(runes, i)
	}

	return tokens
}

// skipToken advances past a single CamelCase token starting at position i and
// returns the index of the first character of the next token.
//
// Rules:
//   - A run of uppercase letters followed by a lowercase letter is an acronym
//     token (e.g. HTTP, XML). The final uppercase letter begins the next token
//     when followed by lowercase (XMLParser → XML + Parser).
//   - A single uppercase letter followed by lowercase letters is a regular token
//     (e.g. Parse, Client).
//   - A run of lowercase letters or digits continues the current token.
func skipToken(runes []rune, i int) int {
	n := len(runes)

	if unicode.IsUpper(runes[i]) {
		return skipUpperToken(runes, i, n)
	}

	// Lowercase or digit — advance until an uppercase letter or underscore.
	for i < n && runes[i] != '_' && !unicode.IsUpper(runes[i]) {
		i++
	}
	return i
}

// skipUpperToken handles a token that starts with an uppercase letter.
func skipUpperToken(runes []rune, i, n int) int {
	// Consume the leading uppercase letter.
	i++

	// If next is lowercase, this is a standard CamelCase token (e.g. Parse).
	if i < n && unicode.IsLower(runes[i]) {
		for i < n && runes[i] != '_' && !unicode.IsUpper(runes[i]) {
			i++
		}
		return i
	}

	// Next is uppercase or end: consume the acronym run.
	// Stop one short when the run is followed by a lowercase letter so that
	// the last uppercase letter is left as the start of the next token.
	// Example: XMLParser → consume XML, leave P.
	for i < n && unicode.IsUpper(runes[i]) {
		if i+1 < n && unicode.IsLower(runes[i+1]) {
			break
		}
		i++
	}
	return i
}
