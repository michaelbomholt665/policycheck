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

	if c.cfg.Documentation.EnforceHeaders {
		*c.viols = append(*c.viols, checkFileHeader(path, rel, lang, c.cfg)...)
	}

	if c.cfg.Documentation.EnforceFunctions {
		*c.viols = append(*c.viols, checkFunctionDocumentation(c.ctx, c.root, path, rel, lang, c.cfg)...)
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
func checkFileHeader(path, rel string, lang documentationLanguage, cfg config.PolicyConfig) []types.Violation {
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

	if lang.name == "python" {
		shebangLine, shebangViols := checkPythonShebang(lines, rel, lang, cfg)
		viols = append(viols, shebangViols...)
		headerStartLine = shebangLine
	}

	match, matchViols := locateHeaderPath(lines, rel, lang, headerStartLine, cfg)
	viols = append(viols, matchViols...)
	if match.lineIndex < 0 {
		return viols
	}

	viols = append(viols, checkHeaderDescription(lines, rel, lang, match.lineIndex, cfg)...)
	return viols
}

// checkPythonShebang ensures executable python scripts have the correct shebang line.
func checkPythonShebang(
	lines []string,
	rel string,
	lang documentationLanguage,
	cfg config.PolicyConfig,
) (int, []types.Violation) {
	if !requiresPythonShebang(rel, cfg) {
		if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "#!") {
			return 1, nil
		}
		return 0, nil
	}

	if strings.TrimSpace(lines[0]) == pythonShebangLine {
		return 1, nil
	}

	return 0, []types.Violation{newDocumentationViolation(
		rel,
		1,
		lang,
		documentationViolationSpec{
			subject:     "missing required shebang",
			expectation: fmt.Sprintf("expected %q on line 1 for files under configured executable roots", pythonShebangLine),
		},
		cfg.Documentation.Level,
	)}
}

// requiresPythonShebang returns true if the script is in a designated executable root.
func requiresPythonShebang(rel string, cfg config.PolicyConfig) bool {
	if cfg.Documentation.Level != "strict" || !cfg.Documentation.RequireShebangPython {
		return false
	}

	return host.HasPrefix(rel, cfg.Documentation.PythonShebangRoots)
}

// locateHeaderPath finds the repo-relative path header within the search window.
func locateHeaderPath(
	lines []string,
	rel string,
	lang documentationLanguage,
	startLine int,
	cfg config.PolicyConfig,
) (headerMatch, []types.Violation) {
	if cfg.Documentation.Level == "strict" {
		return locateStrictHeaderPath(headerSearchParams{
			lines:     lines,
			rel:       rel,
			lang:      lang,
			startLine: startLine,
			level:     cfg.Documentation.Level,
		})
	}

	return locateLooseHeaderPath(headerSearchParams{
		lines:     lines,
		rel:       rel,
		lang:      lang,
		startLine: startLine,
		level:     cfg.Documentation.Level,
	})
}

// locateStrictHeaderPath expects the path header exactly on the designated start line.
func locateStrictHeaderPath(p headerSearchParams) (headerMatch, []types.Violation) {
	if p.startLine >= len(p.lines) {
		return headerMatch{lineIndex: -1}, []types.Violation{newDocumentationViolation(
			p.rel,
			p.startLine+1,
			p.lang,
			documentationViolationSpec{
				subject:     "file header path is missing",
				expectation: fmt.Sprintf("expected %q on line %d (end of file reached)", p.rel, p.startLine+1),
			},
			p.level,
		)}
	}

	foundPath := extractHeaderPath(p.lines[p.startLine], p.lang.commentPrefix)
	if foundPath == p.rel {
		return headerMatch{lineIndex: p.startLine, foundPath: foundPath}, nil
	}

	actualText := ""
	if foundPath != "" {
		actualText = fmt.Sprintf(", found %q", foundPath)
	}

	return headerMatch{lineIndex: -1, foundPath: foundPath}, []types.Violation{newDocumentationViolation(
		p.rel,
		p.startLine+1,
		p.lang,
		documentationViolationSpec{
			subject:     "file header path is incorrect",
			expectation: fmt.Sprintf("expected %q on line %d%s", p.rel, p.startLine+1, actualText),
		},
		p.level,
	)}
}

type headerSearchParams struct {
	lines     []string
	rel       string
	lang      documentationLanguage
	startLine int
	level     string
}

// locateLooseHeaderPath searches for the path header within a sliding window.
func locateLooseHeaderPath(p headerSearchParams) (headerMatch, []types.Violation) {
	maxLine := p.startLine + headerSearchWindow
	if maxLine > len(p.lines) {
		maxLine = len(p.lines)
	}

	for idx := p.startLine; idx < maxLine; idx++ {
		foundPath := extractHeaderPath(p.lines[idx], p.lang.commentPrefix)
		if foundPath == p.rel {
			return headerMatch{lineIndex: idx, foundPath: foundPath}, nil
		}
	}

	return headerMatch{lineIndex: -1}, []types.Violation{newDocumentationViolation(
		p.rel,
		p.startLine+1,
		p.lang,
		documentationViolationSpec{
			subject:      "file header path is missing",
			expectation:  fmt.Sprintf("expected %q within the first %d header lines", p.rel, headerSearchWindow),
			functionName: "",
		},
		p.level,
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
	rel string,
	lang documentationLanguage,
	headerLine int,
	cfg config.PolicyConfig,
) []types.Violation {
	descriptionCount := 0
	for idx := headerLine + 1; idx < len(lines); idx++ {
		descriptionLine := extractHeaderPath(lines[idx], lang.commentPrefix)
		if descriptionLine == "" {
			break
		}
		descriptionCount++
	}

	if descriptionCount >= minDescriptionLineCount && descriptionCount <= maxDescriptionLineCount {
		return nil
	}

	return []types.Violation{newDocumentationViolation(
		rel,
		headerLine+2,
		lang,
		documentationViolationSpec{
			subject:     fmt.Sprintf("module description has %d line(s)", descriptionCount),
			expectation: fmt.Sprintf("expected %d-%d comment lines immediately after the path header", minDescriptionLineCount, maxDescriptionLineCount),
		},
		cfg.Documentation.Level,
	)}
}

// checkFunctionDocumentation dispatches function-level checks for non-Go files.
func checkFunctionDocumentation(
	ctx context.Context,
	root, path, rel string,
	lang documentationLanguage,
	cfg config.PolicyConfig,
) []types.Violation {
	if lang.name == "go" {
		return checkGoFunctionDocumentation(path, rel, lang, cfg)
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

		viols = append(viols, validateScannedFunctionDocumentation(rel, lang, cfg, fact)...)
	}

	return viols
}

// checkGoFunctionDocumentation performs AST-based documentation checks for Go functions.
func checkGoFunctionDocumentation(
	path, rel string,
	lang documentationLanguage,
	cfg config.PolicyConfig,
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

		if fnViols := inspectGoFunction(fn, fset, rel, lang, cfg); len(fnViols) > 0 {
			viols = append(viols, fnViols...)
		}
		return true
	})

	return viols
}

// inspectGoFunction validates the documentation presence and style for a single Go function.
func inspectGoFunction(fn *ast.FuncDecl, fset *token.FileSet, rel string, lang documentationLanguage, cfg config.PolicyConfig) []types.Violation {
	line := fset.Position(fn.Pos()).Line
	name := fn.Name.Name

	if fn.Doc == nil || strings.TrimSpace(fn.Doc.Text()) == "" {
		return []types.Violation{newDocumentationViolation(
			rel,
			line,
			lang,
			documentationViolationSpec{
				subject:      fmt.Sprintf("%s %q is missing documentation", functionSubject(fn), name),
				expectation:  fmt.Sprintf("expected a doc comment immediately above the %s", functionSubject(fn)),
				functionName: name,
			},
			cfg.Documentation.Level,
		)}
	}

	if cfg.Documentation.Level == "loose" || cfg.Documentation.GoStyle == "presence_only" {
		return nil
	}

	return verifyGoFunctionStyle(goStyleParams{
		fn:      fn,
		fset:    fset,
		rel:     rel,
		lang:    lang,
		level:   cfg.Documentation.Level,
		goStyle: cfg.Documentation.GoStyle,
	})
}

type goStyleParams struct {
	fn      *ast.FuncDecl
	fset    *token.FileSet
	rel     string
	lang    documentationLanguage
	level   string
	goStyle string
}

// verifyGoFunctionStyle performs deep style checks for a Go function that has documentation.
func verifyGoFunctionStyle(p goStyleParams) []types.Violation {
	var viols []types.Violation
	name := p.fn.Name.Name
	docText := strings.TrimSpace(p.fn.Doc.Text())
	summary := firstNonEmptyLine(docText)

	if p.goStyle == "google" && !strings.HasPrefix(summary, name) {
		viols = append(viols, newDocumentationViolation(
			p.rel,
			p.fset.Position(p.fn.Doc.Pos()).Line,
			p.lang,
			documentationViolationSpec{
				subject:      fmt.Sprintf("%s %q violates documentation style", functionSubject(p.fn), name),
				expectation:  fmt.Sprintf("expected the summary line to start with %q", name),
				functionName: name,
			},
			p.level,
		))
	}

	if floorReason := validateSummaryQuality(summary, name, true); floorReason != "" {
		viols = append(viols, newDocumentationViolation(
			p.rel,
			p.fset.Position(p.fn.Doc.Pos()).Line,
			p.lang,
			documentationViolationSpec{
				subject:      fmt.Sprintf("%s %q violates documentation style", functionSubject(p.fn), name),
				expectation:  floorReason,
				functionName: name,
			},
			p.level,
		))
	}

	if p.goStyle == "google" && needsGoBlankSeparator(p.fn.Doc) {
		viols = append(viols, newDocumentationViolation(
			p.rel,
			p.fset.Position(p.fn.Doc.Pos()).Line,
			p.lang,
			documentationViolationSpec{
				subject:      fmt.Sprintf("%s %q violates documentation style", functionSubject(p.fn), name),
				expectation:  "expected a blank comment separator line before an additional paragraph",
				functionName: name,
			},
			p.level,
		))
	}

	return viols
}

// validateScannedFunctionDocumentation validates the docstring of a scanned symbol.
func validateScannedFunctionDocumentation(
	rel string,
	lang documentationLanguage,
	cfg config.PolicyConfig,
	fact types.PolicyFact,
) []types.Violation {
	if strings.TrimSpace(fact.Docstring) == "" {
		return []types.Violation{newDocumentationViolation(
			rel,
			fact.LineNumber,
			lang,
			documentationViolationSpec{
				subject:      fmt.Sprintf("%s %q is missing documentation", fact.SymbolKind, fact.SymbolName),
				expectation:  fmt.Sprintf("expected attached documentation immediately above the %s", fact.SymbolKind),
				functionName: fact.SymbolName,
			},
			cfg.Documentation.Level,
		)}
	}

	if cfg.Documentation.Level == "loose" {
		return nil
	}

	switch lang.name {
	case "python":
		return validatePythonStrictDocumentation(rel, lang, cfg, fact)
	case "typescript":
		return validateTypeScriptStrictDocumentation(rel, lang, cfg, fact)
	default:
		return nil
	}
}

// validatePythonStrictDocumentation delegates to specific Python docstring style validators.
func validatePythonStrictDocumentation(
	rel string,
	lang documentationLanguage,
	cfg config.PolicyConfig,
	fact types.PolicyFact,
) []types.Violation {
	style := cfg.Documentation.PythonStyle
	if style == "presence_only" {
		return nil
	}

	summary := firstNonEmptyLine(fact.Docstring)
	if style == "standard" {
		return validateStrictSummaryFloor(rel, lang, cfg, fact, summary)
	}

	if viols := validateStrictSummaryFloor(rel, lang, cfg, fact, summary); len(viols) > 0 {
		return viols
	}

	switch style {
	case "google":
		return validatePythonGoogleDoc(rel, lang, cfg, fact)
	case "numpy":
		return validatePythonNumpyDoc(rel, lang, cfg, fact)
	case "restructuredtext":
		return validatePythonRESTDoc(rel, lang, cfg, fact)
	default:
		return nil
	}
}

// validateTypeScriptStrictDocumentation validates TSDoc blocks in TypeScript files.
func validateTypeScriptStrictDocumentation(
	rel string,
	lang documentationLanguage,
	cfg config.PolicyConfig,
	fact types.PolicyFact,
) []types.Violation {
	style := cfg.Documentation.TypeScriptStyle
	if style == "presence_only" {
		return nil
	}

	summary := firstTypeScriptSummary(fact.Docstring)
	if style == "standard" {
		return validateStrictSummaryFloor(rel, lang, cfg, fact, summary)
	}

	if !strings.HasPrefix(strings.TrimSpace(fact.Docstring), "/**") {
		return []types.Violation{newStyleViolation(
			rel,
			fact.LineNumber,
			lang,
			cfg.Documentation.Level,
			fact,
			"expected a /** ... */ documentation block immediately above the function",
		)}
	}

	if viols := validateStrictSummaryFloor(rel, lang, cfg, fact, summary); len(viols) > 0 {
		return viols
	}

	return validateTypeScriptTagSections(rel, lang, cfg, fact)
}

// validatePythonGoogleDoc validates presence of Args: and Returns: in Google style.
func validatePythonGoogleDoc(
	rel string,
	lang documentationLanguage,
	cfg config.PolicyConfig,
	fact types.PolicyFact,
) []types.Violation {
	var viols []types.Violation
	if len(fact.Params) > 0 && !googleArgsRegex.MatchString(fact.Docstring) {
		viols = append(viols, newStyleViolation(rel, fact.LineNumber, lang, cfg.Documentation.Level, fact, `missing required "Args:" section`))
	}

	if !googleReturnsRegex.MatchString(fact.Docstring) {
		viols = append(viols, newStyleViolation(rel, fact.LineNumber, lang, cfg.Documentation.Level, fact, `missing required "Returns:" section`))
	}

	return viols
}

// validatePythonNumpyDoc validates presence of Parameters and Returns in Numpy style.
func validatePythonNumpyDoc(
	rel string,
	lang documentationLanguage,
	cfg config.PolicyConfig,
	fact types.PolicyFact,
) []types.Violation {
	var viols []types.Violation
	if len(fact.Params) > 0 && !numpyParametersRegex.MatchString(fact.Docstring) {
		viols = append(viols, newStyleViolation(rel, fact.LineNumber, lang, cfg.Documentation.Level, fact, `missing required "Parameters" section`))
	}

	if !numpyReturnsRegex.MatchString(fact.Docstring) {
		viols = append(viols, newStyleViolation(rel, fact.LineNumber, lang, cfg.Documentation.Level, fact, `missing required "Returns" section`))
	}

	return viols
}

// validatePythonRESTDoc validates presence of :param: and :returns: in reST style.
func validatePythonRESTDoc(
	rel string,
	lang documentationLanguage,
	cfg config.PolicyConfig,
	fact types.PolicyFact,
) []types.Violation {
	var viols []types.Violation
	for _, param := range fact.Params {
		if !strings.Contains(fact.Docstring, ":param "+param+":") {
			viols = append(viols, newStyleViolation(rel, fact.LineNumber, lang, cfg.Documentation.Level, fact, fmt.Sprintf("missing :param field for argument %q", param)))
		}
	}

	if !strings.Contains(fact.Docstring, ":returns:") && !strings.Contains(fact.Docstring, ":return:") {
		viols = append(viols, newStyleViolation(rel, fact.LineNumber, lang, cfg.Documentation.Level, fact, "missing :returns: field"))
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

// firstTypeScriptSummary extracts the first meaningful summary line from a TSDoc block.
func firstTypeScriptSummary(docstring string) string {
	for _, line := range strings.Split(docstring, "\n") {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimPrefix(trimmed, "/**")
		trimmed = strings.TrimPrefix(trimmed, "*/")
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "*"))
		if trimmed == "" || strings.HasPrefix(trimmed, "@") {
			continue
		}
		return trimmed
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

// validateStrictSummaryFloor checks that a summary meets minimum length and quality rules.
func validateStrictSummaryFloor(
	rel string,
	lang documentationLanguage,
	cfg config.PolicyConfig,
	fact types.PolicyFact,
	summary string,
) []types.Violation {
	floorReason := validateSummaryQuality(summary, fact.SymbolName, false)
	if floorReason == "" {
		return nil
	}

	return []types.Violation{newStyleViolation(
		rel,
		fact.LineNumber,
		lang,
		cfg.Documentation.Level,
		fact,
		floorReason,
	)}
}

// validateTypeScriptTagSections ensures required TSDoc tags like @param are present.
func validateTypeScriptTagSections(
	rel string,
	lang documentationLanguage,
	cfg config.PolicyConfig,
	fact types.PolicyFact,
) []types.Violation {
	var viols []types.Violation
	for _, param := range fact.Params {
		if strings.Contains(fact.Docstring, "@param "+param) {
			continue
		}

		viols = append(viols, newStyleViolation(
			rel,
			fact.LineNumber,
			lang,
			cfg.Documentation.Level,
			fact,
			fmt.Sprintf("missing @param tag for argument %q", param),
		))
	}

	if strings.Contains(fact.Docstring, "@returns") || strings.Contains(fact.Docstring, "@return") {
		return viols
	}

	return append(viols, newStyleViolation(
		rel,
		fact.LineNumber,
		lang,
		cfg.Documentation.Level,
		fact,
		"missing required @returns tag",
	))
}

// newStyleViolation creates a documentation violation with specific style information.
func newStyleViolation(
	file string,
	line int,
	lang documentationLanguage,
	level string,
	fact types.PolicyFact,
	expectation string,
) types.Violation {
	return newDocumentationViolation(
		file,
		line,
		lang,
		documentationViolationSpec{
			subject:      fmt.Sprintf("%s %q violates documentation style", fact.SymbolKind, fact.SymbolName),
			expectation:  expectation,
			functionName: fact.SymbolName,
		},
		level,
	)
}

// newDocumentationViolation creates a standardized violation for documentation hygiene issues.
func newDocumentationViolation(
	file string,
	line int,
	lang documentationLanguage,
	spec documentationViolationSpec,
	level string,
) types.Violation {
	message := fmt.Sprintf("%s (level=%s, %s=%s); %s [%s]", spec.subject, level, lang.styleKey, lang.styleValue, spec.expectation, documentationRuleID)
	return types.Violation{
		RuleID:   documentationRuleID,
		File:     file,
		Line:     line,
		Function: spec.functionName,
		Message:  message,
		Severity: "error",
	}
}
