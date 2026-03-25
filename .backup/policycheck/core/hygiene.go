// internal/policycheck/core/hygiene.go
// Enforces naming conventions and Google-style doc comments for Go symbols.

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
	"strings"

	"policycheck/internal/policycheck/types"
	"policycheck/internal/policycheck/utils"
)

// CheckSymbolCommentPolicies validates exported symbols and functions have required doc comments.
func CheckSymbolCommentPolicies(root string) []types.Violation {
	violations := []types.Violation{}
	fset := token.NewFileSet()

	bases := []string{
		filepath.Join(root, "internal"),
		filepath.Join(root, "cmd"),
	}

	for _, base := range bases {
		violations = append(violations, collectCommentPolicyViolations(root, base, fset)...)
	}
	return violations
}

// collectCommentPolicyViolations walks a base tree and collects symbol comment violations.
func collectCommentPolicyViolations(root, base string, fset *token.FileSet) []types.Violation {
	violations := []types.Violation{}
	_ = filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
		if violation, skip := buildWalkErrorViolation(base, path, walkErr); skip {
			if violation.Message != "" {
				violations = append(violations, violation)
			}
			return nil
		}
		if shouldSkipCommentCheck(root, path, entry) {
			return nil
		}

		rel := utils.RelOrAbs(root, path)
		violations = append(violations, checkFileComments(fset, path, rel)...)
		return nil
	})
	return violations
}

// buildWalkErrorViolation converts a WalkDir error into a policy violation when appropriate.
func buildWalkErrorViolation(base, path string, walkErr error) (types.Violation, bool) {
	if walkErr == nil {
		return types.Violation{}, false
	}
	if os.IsNotExist(walkErr) && path == base {
		return types.Violation{}, true
	}
	return types.Violation{
		Path:    path,
		Message: fmt.Sprintf("error walking directory for comment checks: %v", walkErr),
	}, true
}

// shouldSkipCommentCheck reports whether a file should be excluded from symbol comment checks.
func shouldSkipCommentCheck(root, path string, entry fs.DirEntry) bool {
	if entry.IsDir() || filepath.Ext(path) != ".go" {
		return true
	}

	rel := utils.RelOrAbs(root, path)
	if strings.HasPrefix(rel, "cmd/policycheck") {
		return true
	}
	return isGeneratedOrTestGoFile(entry.Name())
}

// isGeneratedOrTestGoFile reports whether the file name is excluded from comment checks.
func isGeneratedOrTestGoFile(name string) bool {
	return strings.HasSuffix(name, "_test.go") ||
		strings.HasPrefix(name, "zz_generated") ||
		strings.HasSuffix(name, ".gen.go") ||
		strings.HasSuffix(name, "_mock.go")
}

// checkFileComments parses a Go file and checks all its declarations for doc comment compliance.
func checkFileComments(fset *token.FileSet, path, rel string) []types.Violation {
	violations := []types.Violation{}
	fileNode, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		violations = append(violations, types.Violation{Path: rel, Message: fmt.Sprintf("unable to parse file for comment checks: %v", err)})
		return violations
	}
	for _, decl := range fileNode.Decls {
		violations = append(violations, checkDeclComments(rel, decl)...)
	}
	return violations
}

// checkDeclComments routes a declaration to the appropriate comment checker based on its type.
func checkDeclComments(rel string, decl ast.Decl) []types.Violation {
	switch typed := decl.(type) {
	case *ast.FuncDecl:
		return checkFuncComments(rel, typed)
	case *ast.GenDecl:
		return checkGenDeclComments(rel, typed)
	default:
		return nil
	}
}

// checkFuncComments validates that a function has a proper name and Google-style doc comment.
func checkFuncComments(rel string, decl *ast.FuncDecl) []types.Violation {
	if decl.Name == nil {
		return nil
	}
	violations := []types.Violation{}
	if !hasMultipleTokens(decl.Name.Name) {
		violations = append(violations, types.Violation{
			Path:    rel,
			Message: fmt.Sprintf("function %s must have a minimum of 2 tokens in its name (e.g. ValidateSchema instead of validate)", decl.Name.Name),
		})
	}
	if !hasDocWithPrefix(decl.Doc, decl.Name.Name) {
		violations = append(violations, types.Violation{
			Path:    rel,
			Message: fmt.Sprintf("function %s must have a Google-style doc comment starting with its name", decl.Name.Name),
		})
	}
	return violations
}

// checkGenDeclComments checks all specifications in a generic declaration for doc comment compliance.
func checkGenDeclComments(rel string, decl *ast.GenDecl) []types.Violation {
	violations := []types.Violation{}
	for _, spec := range decl.Specs {
		violations = append(violations, checkSpecComments(rel, decl.Doc, spec)...)
	}
	return violations
}

// checkSpecComments routes a specification to the appropriate comment checker based on its type.
func checkSpecComments(rel string, declDoc *ast.CommentGroup, spec ast.Spec) []types.Violation {
	switch typed := spec.(type) {
	case *ast.TypeSpec:
		return checkTypeSpecComments(rel, declDoc, typed)
	case *ast.ValueSpec:
		return checkValueSpecComments(rel, declDoc, typed)
	default:
		return nil
	}
}

// checkTypeSpecComments validates that an exported type has a Google-style doc comment.
func checkTypeSpecComments(rel string, declDoc *ast.CommentGroup, spec *ast.TypeSpec) []types.Violation {
	if !spec.Name.IsExported() {
		return nil
	}
	doc := resolveSpecDoc(spec.Doc, declDoc)
	if hasDocWithPrefix(doc, spec.Name.Name) {
		return nil
	}
	return []types.Violation{{Path: rel, Message: fmt.Sprintf("exported type %s must have a doc comment starting with its name", spec.Name.Name)}}
}

// checkValueSpecComments validates that exported value specifications have doc comments.
func checkValueSpecComments(rel string, declDoc *ast.CommentGroup, spec *ast.ValueSpec) []types.Violation {
	doc := resolveSpecDoc(spec.Doc, declDoc)
	violations := []types.Violation{}
	for _, ident := range spec.Names {
		if !ident.IsExported() || hasAnyDoc(doc) {
			continue
		}
		violations = append(violations, types.Violation{
			Path:    rel,
			Message: fmt.Sprintf("exported symbol %s must have a doc comment", ident.Name),
		})
	}
	return violations
}

// resolveSpecDoc returns the specification-level doc comment, falling back to declaration-level if not present.
func resolveSpecDoc(specDoc, declDoc *ast.CommentGroup) *ast.CommentGroup {
	if specDoc != nil {
		return specDoc
	}
	return declDoc
}

// hasAnyDoc returns true if the comment group contains at least one comment.
func hasAnyDoc(doc *ast.CommentGroup) bool {
	return doc != nil && len(doc.List) > 0
}

// hasDocWithPrefix checks if a comment group contains a comment starting with the given prefix.
func hasDocWithPrefix(doc *ast.CommentGroup, prefix string) bool {
	if !hasAnyDoc(doc) {
		return false
	}
	for _, c := range doc.List {
		text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		if strings.HasPrefix(text, prefix+" ") || strings.HasPrefix(text, prefix+"\t") || text == prefix {
			return true
		}
	}
	return false
}

// hasMultipleTokens returns true if a function name has at least 2 tokens (e.g., ValidateSchema).
func hasMultipleTokens(name string) bool {
	if name == "main" {
		return true
	}
	if len(name) < 2 {
		return false
	}
	if strings.Contains(name, "_") {
		return true
	}
	for i := 1; i < len(name); i++ {
		if name[i] >= 'A' && name[i] <= 'Z' {
			return true
		}
	}
	return false
}
