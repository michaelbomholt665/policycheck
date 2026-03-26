// internal/policycheck/core/hygiene/doc_style.go
// Package hygiene/doc_style performs quality checks on symbol-level documentation.
// It verifies word counts, prefixes, and prohibited markers on exported Go symbols.
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

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/host"
	"policycheck/internal/policycheck/types"
	"policycheck/internal/policycheck/utils"
)

// skippedSuffixes lists file name patterns that are excluded from doc style checks.
var skippedSuffixes = []string{
	"_test.go",
	".gen.go",
	"_mock.go",
}

// CheckDocStyle validates that exported symbols have Google-style doc comments
// across all configured scan roots.
func CheckDocStyle(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation {
	walk, err := host.ResolveWalkProvider()
	if err != nil {
		return nil
	}

	scanRoots := resolveScanRoots(cfg)
	var viols []types.Violation

	for _, scanRoot := range scanRoots {
		absRoot := filepath.Join(root, scanRoot)
		walk.WalkDirectoryTree(absRoot, func(path string, d fs.DirEntry, err error) error {
			return collectDocViolations(docStyleContext{
				root:  root,
				path:  path,
				d:     d,
				err:   err,
				cfg:   cfg,
				viols: &viols,
			})
		})
	}

	return viols
}

// docStyleContext holds context parameters for the per-entry walk callback
// and file-level documentation validation functions.
type docStyleContext struct {
	root  string
	path  string
	d     fs.DirEntry
	err   error
	cfg   config.PolicyConfig
	viols *[]types.Violation

	// For validateSymbolDoc
	name          string
	kind          string
	doc           *ast.CommentGroup
	line          int
	requirePrefix bool
}

// collectDocViolations filters irrelevant tree walking entries before delegating
// to the appropriate file-level documentation checks.
func collectDocViolations(sctx docStyleContext) error {
	if sctx.err != nil || sctx.d.IsDir() || filepath.Ext(sctx.path) != ".go" {
		return nil
	}
	if isSkippedFile(sctx.path) {
		return nil
	}
	rel := utils.ToSlashRel(sctx.root, sctx.path)
	if isExcluded(rel, sctx.cfg.Hygiene.ExcludePrefixes) {
		return nil
	}
	*sctx.viols = append(*sctx.viols, checkFileDocStyle(sctx.root, sctx.path)...)
	return nil
}

// isSkippedFile returns true for generated files and test files that do not
// require doc comment coverage.
func isSkippedFile(path string) bool {
	base := filepath.Base(path)
	if strings.HasPrefix(base, "zz_generated") {
		return true
	}
	for _, suffix := range skippedSuffixes {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return false
}

// checkFileDocStyle parses a single Go file and validates doc comments on all
// exported symbols.
func checkFileDocStyle(root, path string) []types.Violation {
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
	var viols []types.Violation

	ast.Inspect(f, func(n ast.Node) bool {
		viols = append(viols, inspectDocNode(n, rel, fset)...)
		return true
	})

	return viols
}

// inspectDocNode dispatches AST node types to the appropriate doc check.
func inspectDocNode(n ast.Node, rel string, fset *token.FileSet) []types.Violation {
	switch decl := n.(type) {
	case *ast.FuncDecl:
		return checkFuncDoc(decl, rel, fset)
	case *ast.GenDecl:
		return checkGenDeclDoc(decl, rel, fset)
	}
	return nil
}

// checkFuncDoc validates the doc comment on an exported function declaration.
func checkFuncDoc(decl *ast.FuncDecl, rel string, fset *token.FileSet) []types.Violation {
	if decl.Name == nil || !decl.Name.IsExported() {
		return nil
	}
	name := decl.Name.Name
	line := fset.Position(decl.Pos()).Line
	return validateSymbolDoc(docStyleContext{
		path:          rel,
		name:          name,
		kind:          "function",
		doc:           decl.Doc,
		line:          line,
		requirePrefix: true,
	})
}

// checkGenDeclDoc validates doc comments on exported type and value declarations.
func checkGenDeclDoc(decl *ast.GenDecl, rel string, fset *token.FileSet) []types.Violation {
	var viols []types.Violation
	for _, spec := range decl.Specs {
		viols = append(viols, checkSpecDoc(spec, decl.Doc, rel, fset)...)
	}
	return viols
}

// checkSpecDoc validates doc comments on a single spec within a GenDecl.
func checkSpecDoc(spec ast.Spec, declDoc *ast.CommentGroup, rel string, fset *token.FileSet) []types.Violation {
	switch typed := spec.(type) {
	case *ast.TypeSpec:
		return checkTypeDoc(typed, declDoc, rel, fset)
	case *ast.ValueSpec:
		return checkValueDoc(typed, declDoc, rel, fset)
	}
	return nil
}

// checkTypeDoc validates the doc comment on an exported type spec.
func checkTypeDoc(spec *ast.TypeSpec, declDoc *ast.CommentGroup, rel string, fset *token.FileSet) []types.Violation {
	if spec.Name == nil || !spec.Name.IsExported() {
		return nil
	}
	doc := resolveSpecDoc(spec.Doc, declDoc)
	line := fset.Position(spec.Pos()).Line
	return validateSymbolDoc(docStyleContext{
		path:          rel,
		name:          spec.Name.Name,
		kind:          "type",
		doc:           doc,
		line:          line,
		requirePrefix: true,
	})
}

// checkValueDoc validates doc comments on exported variable and constant specs.
func checkValueDoc(spec *ast.ValueSpec, declDoc *ast.CommentGroup, rel string, fset *token.FileSet) []types.Violation {
	doc := resolveSpecDoc(spec.Doc, declDoc)
	var viols []types.Violation
	for _, name := range spec.Names {
		if !name.IsExported() {
			continue
		}
		line := fset.Position(name.Pos()).Line
		viols = append(viols, validateSymbolDoc(docStyleContext{
			path:          rel,
			name:          name.Name,
			kind:          "symbol",
			doc:           doc,
			line:          line,
			requirePrefix: false,
		})...)
	}
	return viols
}

// validateSymbolDoc runs presence and quality checks against a symbol's doc comment.
func validateSymbolDoc(sctx docStyleContext) []types.Violation {
	if sctx.requirePrefix && !HasDocWithPrefix(sctx.doc, sctx.name) {
		return []types.Violation{missingPrefixViolation(sctx.path, sctx.name, sctx.kind, sctx.line)}
	}
	if !sctx.requirePrefix && !HasAnyDoc(sctx.doc) {
		return []types.Violation{missingDocViolation(sctx.path, sctx.name, sctx.line)}
	}
	return checkDocQuality(sctx.path, sctx.name, sctx.doc, sctx.line)
}

// checkDocQuality validates the content of an existing doc comment for word
// count and the absence of TODO/FIXME markers. // NOSONAR
func checkDocQuality(rel, name string, doc *ast.CommentGroup, line int) []types.Violation {
	if doc == nil {
		return nil
	}

	var viols []types.Violation
	var words []string

	for _, c := range doc.List {
		text := stripCommentMarkers(c.Text)
		if containsTodoMarker(text) {
			viols = append(viols, todoViolation(rel, name, line))
		}
		words = append(words, strings.Fields(text)...)
	}

	if len(words) < 5 {
		viols = append(viols, shortDocViolation(rel, name, len(words), line))
	}

	return viols
}

// --- comment text helpers ---

// stripCommentMarkers removes leading // and /* */ markers from a raw comment string.
func stripCommentMarkers(raw string) string {
	text := strings.TrimSpace(strings.TrimPrefix(raw, "//"))
	text = strings.TrimSpace(strings.TrimPrefix(text, "/*"))
	return strings.TrimSpace(strings.TrimSuffix(text, "*/"))
}

// containsTodoMarker returns true when text contains a TODO or FIXME annotation. //nolint:S1135
func containsTodoMarker(text string) bool {
	upper := strings.ToUpper(text)
	return strings.Contains(upper, "TODO") || strings.Contains(upper, "FIXME")
}

// resolveSpecDoc returns specDoc when set, falling back to the parent declDoc.
func resolveSpecDoc(specDoc, declDoc *ast.CommentGroup) *ast.CommentGroup {
	if specDoc != nil {
		return specDoc
	}
	return declDoc
}

// HasAnyDoc returns true if the comment group contains at least one comment.
func HasAnyDoc(doc *ast.CommentGroup) bool {
	return doc != nil && len(doc.List) > 0
}

// HasDocWithPrefix returns true when the comment group contains a comment whose
// text begins with the given prefix followed by a space, tab, or is exactly the prefix.
func HasDocWithPrefix(doc *ast.CommentGroup, prefix string) bool {
	if !HasAnyDoc(doc) {
		return false
	}
	for _, c := range doc.List {
		text := stripCommentMarkers(c.Text)
		if strings.HasPrefix(text, prefix+" ") ||
			strings.HasPrefix(text, prefix+"\t") ||
			text == prefix {
			return true
		}
	}
	return false
}

// --- violation constructors ---

// missingPrefixViolation returns a violation for a symbol missing a prefixed doc comment.
func missingPrefixViolation(rel, name, kind string, line int) types.Violation {
	return types.Violation{
		RuleID:   "hygiene.doc_style",
		File:     rel,
		Function: name,
		Line:     line,
		Message:  fmt.Sprintf("exported %s %s must have a doc comment starting with its name", kind, name),
		Severity: "error",
	}
}

// missingDocViolation returns a violation for a symbol with no doc comment at all.
func missingDocViolation(rel, name string, line int) types.Violation {
	return types.Violation{
		RuleID:   "hygiene.doc_style",
		File:     rel,
		Function: name,
		Line:     line,
		Message:  fmt.Sprintf("exported symbol %s must have a doc comment", name),
		Severity: "error",
	}
}

// todoViolation returns a violation for a TODO or FIXME found in a doc comment. // NOSONAR S1135
func todoViolation(rel, name string, line int) types.Violation {
	return types.Violation{
		RuleID:   "hygiene.doc_style",
		File:     rel,
		Function: name,
		Line:     line,
		Message:  fmt.Sprintf("exported symbol %s doc comment contains TODO or FIXME", name),
		Severity: "error",
	}
}

// shortDocViolation returns a violation for a doc comment below the 5-word minimum.
func shortDocViolation(rel, name string, wordCount, line int) types.Violation {
	return types.Violation{
		RuleID:   "hygiene.doc_style",
		File:     rel,
		Function: name,
		Line:     line,
		Message:  fmt.Sprintf("exported symbol %s doc comment has %d word(s), minimum is 5", name, wordCount),
		Severity: "error",
	}
}
