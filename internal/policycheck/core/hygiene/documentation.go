// internal/policycheck/core/hygiene/documentation.go
// Package hygiene provides code hygiene and documentation policy checks.
// It handles cross-language verification of file headers and docstring quality.
// Supports Go, Python (Numpy/Google/reST), and TypeScript (TSDoc).
package hygiene

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/host"
	"policycheck/internal/policycheck/types"
	"policycheck/internal/policycheck/utils"
)

const (
	documentationRuleID     = "hygiene.documentation"
	headerSearchWindow      = 6
	pythonShebangLine       = "#!/usr/bin/env python3"
	minDescriptionLineCount = 2
	maxDescriptionLineCount = 5
	minSummaryLengthRunes   = 10
)

var (
	numpyParametersRegex = regexp.MustCompile(`(?m)^Parameters\s*\n-+\s*$`)
	numpyReturnsRegex    = regexp.MustCompile(`(?m)^Returns\s*\n-+\s*$`)
	googleArgsRegex      = regexp.MustCompile(`(?m)^Args:\s*$`)
	googleReturnsRegex   = regexp.MustCompile(`(?m)^(Returns|Yields):\s*$`)
	weakSummaryOpeners   = []string{"this function", "this method", "function to"}
)

type documentationLanguage struct {
	name          string
	styleKey      string
	styleValue    string
	commentPrefix string
}

// docContext groups common parameters used throughout documentation checks.
type docContext struct {
	rel  string
	lang documentationLanguage
	cfg  config.PolicyConfig
}

type headerMatch struct {
	lineIndex int
	foundPath string
}

type documentationChecker struct {
	ctx   context.Context
	root  string
	cfg   config.PolicyConfig
	viols *[]types.Violation
}

type documentationViolationSpec struct {
	subject      string
	expectation  string
	functionName string
}

// CheckDocumentation validates file headers and function documentation across languages.
func CheckDocumentation(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation {
	if !cfg.Documentation.Enabled {
		return nil
	}

	walk, err := host.ResolveWalkProvider()
	if err != nil {
		return nil
	}

	var viols []types.Violation
	checker := documentationChecker{
		ctx:   ctx,
		root:  root,
		cfg:   cfg,
		viols: &viols,
	}
	for _, scanRoot := range cfg.Documentation.ScanRoots {
		absRoot := filepath.Join(root, scanRoot)
		_ = walk.WalkDirectoryTree(absRoot, checker.collectViolations)
	}

	return viols
}

// collectViolations is the callback function for directory tree walking.
func (c documentationChecker) collectViolations(path string, d fs.DirEntry, walkErr error) error {
	if walkErr != nil || d.IsDir() {
		return nil
	}

	lang, ok := resolveDocumentationLanguage(path, c.cfg)
	if !ok {
		return nil
	}

	rel := utils.ToSlashRel(c.root, path)
	if isDocumentationExcluded(rel) {
		return nil
	}

	docCtx := docContext{
		rel:  rel,
		lang: lang,
		cfg:  c.cfg,
	}

	if c.cfg.Documentation.EnforceHeaders {
		*c.viols = append(*c.viols, checkFileHeader(path, docCtx)...)
	}

	if c.cfg.Documentation.EnforceFunctions {
		*c.viols = append(*c.viols, checkFunctionDocumentation(c.ctx, c.root, path, docCtx)...)
	}

	return nil
}

// isDocumentationExcluded returns true if the path is in a skip-list (router/tests).
func isDocumentationExcluded(rel string) bool {
	return strings.HasPrefix(rel, "internal/router/") ||
		strings.HasPrefix(rel, "internal/tests/") ||
		strings.HasPrefix(rel, "test/")
}

// resolveDocumentationLanguage maps a file extension to its documentation style settings.
func resolveDocumentationLanguage(path string, cfg config.PolicyConfig) (documentationLanguage, bool) {
	switch filepath.Ext(path) {
	case ".go":
		goStyle := cfg.Documentation.GoStyle
		if goStyle == "" {
			goStyle = "google"
		}
		return documentationLanguage{
			name:          "go",
			styleKey:      "go_style",
			styleValue:    goStyle,
			commentPrefix: "//",
		}, true
	case ".py":
		pythonStyle := cfg.Documentation.PythonStyle
		if pythonStyle == "" {
			pythonStyle = "numpy"
		}
		return documentationLanguage{
			name:          "python",
			styleKey:      "python_style",
			styleValue:    pythonStyle,
			commentPrefix: "#",
		}, true
	case ".ts":
		typeScriptStyle := cfg.Documentation.TypeScriptStyle
		if typeScriptStyle == "" {
			typeScriptStyle = "tsdoc"
		}
		return documentationLanguage{
			name:          "typescript",
			styleKey:      "typescript_style",
			styleValue:    typeScriptStyle,
			commentPrefix: "//",
		}, true
	default:
		return documentationLanguage{}, false
	}
}

// checkFileHeader validates the file header path and module description on line 1.
func checkFileHeader(path string, docCtx docContext) []types.Violation {
	content, err := host.ReadFile(path)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 {
		return nil
	}

	var viols []types.Violation
	headerStartLine := 0

	if docCtx.lang.name == "python" {
		shebangLine, shebangViols := checkPythonShebang(lines, docCtx)
		viols = append(viols, shebangViols...)
		headerStartLine = shebangLine
	}

	match, matchViols := locateHeaderPath(lines, headerStartLine, docCtx)
	viols = append(viols, matchViols...)
	if match.lineIndex < 0 {
		return viols
	}

	viols = append(viols, checkHeaderDescription(lines, match.lineIndex, docCtx)...)
	return viols
}

// checkPythonShebang ensures executable python scripts have the correct shebang line.
func checkPythonShebang(
	lines []string,
	docCtx docContext,
) (int, []types.Violation) {
	if !requiresPythonShebang(docCtx) {
		if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "#!") {
			return 1, nil
		}
		return 0, nil
	}

	if strings.TrimSpace(lines[0]) == pythonShebangLine {
		return 1, nil
	}

	return 0, []types.Violation{newDocumentationViolation(
		1,
		documentationViolationSpec{
			subject:     "missing required shebang",
			expectation: fmt.Sprintf("expected %q on line 1 for files under configured executable roots", pythonShebangLine),
		},
		docCtx,
	)}
}

// requiresPythonShebang returns true if the script is in a designated executable root.
func requiresPythonShebang(docCtx docContext) bool {
	if docCtx.cfg.Documentation.Level != "strict" || !docCtx.cfg.Documentation.RequireShebangPython {
		return false
	}

	return host.HasPrefix(docCtx.rel, docCtx.cfg.Documentation.PythonShebangRoots)
}

// locateHeaderPath finds the repo-relative path header within the search window.
func locateHeaderPath(
	lines []string,
	startLine int,
	docCtx docContext,
) (headerMatch, []types.Violation) {
	if docCtx.cfg.Documentation.Level == "strict" {
		return locateStrictHeaderPath(headerSearchParams{
			lines:     lines,
			startLine: startLine,
			docCtx:    docCtx,
		})
	}

	return locateLooseHeaderPath(headerSearchParams{
		lines:     lines,
		startLine: startLine,
		docCtx:    docCtx,
	})
}

// locateStrictHeaderPath expects the path header exactly on the designated start line.
func locateStrictHeaderPath(p headerSearchParams) (headerMatch, []types.Violation) {
	if p.startLine >= len(p.lines) {
		return headerMatch{lineIndex: -1}, []types.Violation{newDocumentationViolation(
			p.startLine+1,
			documentationViolationSpec{
				subject:     "file header path is missing",
				expectation: fmt.Sprintf("expected %q on line %d (end of file reached)", p.docCtx.rel, p.startLine+1),
			},
			p.docCtx,
		)}
	}

	foundPath := extractHeaderPath(p.lines[p.startLine], p.docCtx.lang.commentPrefix)
	if foundPath == p.docCtx.rel {
		return headerMatch{lineIndex: p.startLine, foundPath: foundPath}, nil
	}

	actualText := ""
	if foundPath != "" {
		actualText = fmt.Sprintf(", found %q", foundPath)
	}

	return headerMatch{lineIndex: -1, foundPath: foundPath}, []types.Violation{newDocumentationViolation(
		p.startLine+1,
		documentationViolationSpec{
			subject:     "file header path is incorrect",
			expectation: fmt.Sprintf("expected %q on line %d%s", p.docCtx.rel, p.startLine+1, actualText),
		},
		p.docCtx,
	)}
}

type headerSearchParams struct {
	lines     []string
	startLine int
	docCtx    docContext
}

// locateLooseHeaderPath searches for the path header within a sliding window.
func locateLooseHeaderPath(p headerSearchParams) (headerMatch, []types.Violation) {
	maxLine := p.startLine + headerSearchWindow
	if maxLine > len(p.lines) {
		maxLine = len(p.lines)
	}

	for idx := p.startLine; idx < maxLine; idx++ {
		foundPath := extractHeaderPath(p.lines[idx], p.docCtx.lang.commentPrefix)
		if foundPath == p.docCtx.rel {
			return headerMatch{lineIndex: idx, foundPath: foundPath}, nil
		}
	}

	return headerMatch{lineIndex: -1}, []types.Violation{newDocumentationViolation(
		p.startLine+1,
		documentationViolationSpec{
			subject:      "file header path is missing",
			expectation:  fmt.Sprintf("expected %q within the first %d header lines", p.docCtx.rel, headerSearchWindow),
			functionName: "",
		},
		p.docCtx,
	)}
}

// extractHeaderPath strips the comment prefix and leading/trailing whitespace from a line.
func extractHeaderPath(line, commentPrefix string) string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, commentPrefix) {
		return ""
	}

	return strings.TrimSpace(strings.TrimPrefix(trimmed, commentPrefix))
}

// checkHeaderDescription ensures the module description has the correct line count.
func checkHeaderDescription(
	lines []string,
	headerLine int,
	docCtx docContext,
) []types.Violation {
	descriptionCount := 0
	for idx := headerLine + 1; idx < len(lines); idx++ {
		descriptionLine := extractHeaderPath(lines[idx], docCtx.lang.commentPrefix)
		if descriptionLine == "" {
			break
		}
		descriptionCount++
	}

	if descriptionCount >= minDescriptionLineCount && descriptionCount <= maxDescriptionLineCount {
		return nil
	}

	return []types.Violation{newDocumentationViolation(
		headerLine+2,
		documentationViolationSpec{
			subject:     fmt.Sprintf("module description has %d line(s)", descriptionCount),
			expectation: fmt.Sprintf("expected %d-%d comment lines immediately after the path header", minDescriptionLineCount, maxDescriptionLineCount),
		},
		docCtx,
	)}
}

// checkFunctionDocumentation dispatches function-level checks for non-Go files.
func checkFunctionDocumentation(
	ctx context.Context,
	root, path string,
	docCtx docContext,
) []types.Violation {
	if docCtx.lang.name == "go" {
		return checkGoFunctionDocumentation(path, docCtx)
	}

	scanner, err := host.ResolveScannerProvider()
	if err != nil {
		return nil
	}

	facts, err := scanner.ScanFile(ctx, root, path)
	if err != nil {
		return nil
	}

	var viols []types.Violation
	for _, fact := range facts {
		if fact.SymbolKind != "function" && fact.SymbolKind != "method" {
			continue
		}

		viols = append(viols, validateScannedFunctionDocumentation(fact, docCtx)...)
	}

	return viols
}

// checkGoFunctionDocumentation performs AST-based documentation checks for Go functions.
func checkGoFunctionDocumentation(
	path string,
	docCtx docContext,
) []types.Violation {
	content, err := host.ReadFile(path)
	if err != nil {
		return nil
	}

	fset := token.NewFileSet()
	fileNode, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return nil
	}

	var viols []types.Violation
	ast.Inspect(fileNode, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		if fnViols := inspectGoFunction(fn, fset, docCtx); len(fnViols) > 0 {
			viols = append(viols, fnViols...)
		}
		return true
	})

	return viols
}

// inspectGoFunction validates the documentation presence and style for a single Go function.
func inspectGoFunction(fn *ast.FuncDecl, fset *token.FileSet, docCtx docContext) []types.Violation {
	line := fset.Position(fn.Pos()).Line
	name := fn.Name.Name

	if fn.Doc == nil || strings.TrimSpace(fn.Doc.Text()) == "" {
		return []types.Violation{newDocumentationViolation(
			line,
			documentationViolationSpec{
				subject:      fmt.Sprintf("%s %q is missing documentation", functionSubject(fn), name),
				expectation:  fmt.Sprintf("expected a doc comment immediately above the %s", functionSubject(fn)),
				functionName: name,
			},
			docCtx,
		)}
	}

	if docCtx.cfg.Documentation.Level == "loose" || docCtx.cfg.Documentation.GoStyle == "presence_only" {
		return nil
	}

	return verifyGoFunctionStyle(goStyleParams{
		fn:     fn,
		fset:   fset,
		docCtx: docCtx,
	})
}

type goStyleParams struct {
	fn     *ast.FuncDecl
	fset   *token.FileSet
	docCtx docContext
}

// verifyGoFunctionStyle performs deep style checks for a Go function that has documentation.
func verifyGoFunctionStyle(p goStyleParams) []types.Violation {
	var viols []types.Violation
	name := p.fn.Name.Name
	docText := strings.TrimSpace(p.fn.Doc.Text())
	summary := firstNonEmptyLine(docText)

	if p.docCtx.cfg.Documentation.GoStyle == "google" && !strings.HasPrefix(summary, name) {
		return []types.Violation{newDocumentationViolation(
			p.fset.Position(p.fn.Doc.Pos()).Line,
			documentationViolationSpec{
				subject:      fmt.Sprintf("%s %q violates documentation style", functionSubject(p.fn), name),
				expectation:  fmt.Sprintf("expected the summary line to start with %q", name),
				functionName: name,
			},
			p.docCtx,
		)}
	}

	if floorReason := validateSummaryQuality(summary, name, true); floorReason != "" {
		viols = append(viols, newDocumentationViolation(
			p.fset.Position(p.fn.Doc.Pos()).Line,
			documentationViolationSpec{
				subject:      fmt.Sprintf("%s %q violates documentation style", functionSubject(p.fn), name),
				expectation:  floorReason,
				functionName: name,
			},
			p.docCtx,
		))
	}

	if p.docCtx.cfg.Documentation.GoStyle == "google" && needsGoBlankSeparator(p.fn.Doc) {
		viols = append(viols, newDocumentationViolation(
			p.fset.Position(p.fn.Doc.Pos()).Line,
			documentationViolationSpec{
				subject:      fmt.Sprintf("%s %q violates documentation style", functionSubject(p.fn), name),
				expectation:  "expected a blank comment separator line before an additional paragraph",
				functionName: name,
			},
			p.docCtx,
		))
	}

	return viols
}

// validateSummaryQuality checks word count, openers, and non-triviality of summaries.
func validateSummaryQuality(summary, symbolName string, rejectBareSymbol bool) string {
	trimmed := strings.TrimSpace(summary)
	if trimmed == "" {
		return "expected a non-empty summary line"
	}

	if rejectBareSymbol && trimmed == symbolName {
		return fmt.Sprintf("expected the summary line to add information beyond %q", symbolName)
	}

	if len([]rune(trimmed)) < minSummaryLengthRunes {
		return fmt.Sprintf("expected a summary line with at least %d characters", minSummaryLengthRunes)
	}

	lowerSummary := strings.ToLower(trimmed)
	for _, opener := range weakSummaryOpeners {
		if strings.HasPrefix(lowerSummary, opener) {
			return fmt.Sprintf("expected a summary line that does not start with %q", opener)
		}
	}

	return ""
}

// firstNonEmptyLine returns the first line of text that is not just whitespace.
func firstNonEmptyLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}

// needsGoBlankSeparator returns true if a blank comment line is missing between paragraphs.
func needsGoBlankSeparator(doc *ast.CommentGroup) bool {
	lines := make([]string, 0, len(doc.List))
	for _, comment := range doc.List {
		text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
		lines = append(lines, text)
	}

	if len(lines) < 2 || containsBlankCommentLine(lines) {
		return false
	}

	first := strings.TrimSpace(lines[0])
	second := strings.TrimSpace(lines[1])
	return strings.HasSuffix(first, ".") && beginsLikelyParagraph(second)
}

// containsBlankCommentLine returns true if any line in the slice is empty.
func containsBlankCommentLine(lines []string) bool {
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			return true
		}
	}

	return false
}

// beginsLikelyParagraph returns true if the line starts with a capital letter.
func beginsLikelyParagraph(line string) bool {
	if line == "" {
		return false
	}

	firstRune := []rune(line)[0]
	return strings.ToUpper(string(firstRune)) == string(firstRune)
}

// functionSubject returns "method" or "function" based on receiver presence.
func functionSubject(fn *ast.FuncDecl) string {
	if fn.Recv != nil {
		return "method"
	}
	return "function"
}

// newStyleViolation creates a documentation violation with specific style information.
func newStyleViolation(
	line int,
	fact types.PolicyFact,
	expectation string,
	docCtx docContext,
) types.Violation {
	return newDocumentationViolation(
		line,
		documentationViolationSpec{
			subject:      fmt.Sprintf("%s %q violates documentation style", fact.SymbolKind, fact.SymbolName),
			expectation:  expectation,
			functionName: fact.SymbolName,
		},
		docCtx,
	)
}

// newDocumentationViolation creates a standardized violation for documentation hygiene issues.
func newDocumentationViolation(
	line int,
	spec documentationViolationSpec,
	docCtx docContext,
) types.Violation {
	message := fmt.Sprintf("%s (level=%s, %s=%s); %s [%s]", spec.subject, docCtx.cfg.Documentation.Level, docCtx.lang.styleKey, docCtx.lang.styleValue, spec.expectation, documentationRuleID)
	return types.Violation{
		RuleID:   documentationRuleID,
		File:     docCtx.rel,
		Line:     line,
		Function: spec.functionName,
		Message:  message,
		Severity: "error",
	}
}
