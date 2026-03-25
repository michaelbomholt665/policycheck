package contracts

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
)

const hardcodedRuntimeKnobRuleID = "hardcoded-runtime-knob"

// CheckHardcodedRuntimeKnobs flags hardcoded runtime knobs that should be config-driven.
func CheckHardcodedRuntimeKnobs(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation {
	walk, err := host.ResolveWalkProvider()
	if err != nil {
		return []types.Violation{{
			RuleID:   hardcodedRuntimeKnobRuleID,
			Message:  fmt.Sprintf("resolve walk provider: %v", err),
			Severity: "error",
		}}
	}

	normalizedIgnorePrefixes := normalizePolicyPrefixes(cfg.Paths.HardcodedRuntimeKnobIgnorePath)
	var violations []types.Violation

	for _, scanRoot := range cfg.Paths.HardcodedRuntimeKnobScanRoots {
		base := filepath.Join(root, filepath.FromSlash(scanRoot))
		walkErr := walk.WalkDirectoryTree(base, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if d.IsDir() || filepath.Ext(path) != ".go" {
				return nil
			}

			relPath, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return nil
			}
			relPath = filepath.ToSlash(relPath)
			if host.HasPrefix(relPath, normalizedIgnorePrefixes) {
				return nil
			}

			content, readErr := host.ReadFile(path)
			if readErr != nil {
				return nil
			}

			violations = append(violations, ValidateHardcodedRuntimeKnobs(relPath, content, cfg.HardcodedRuntimeKnob.Identifiers)...)
			return nil
		})
		if walkErr != nil {
			violations = append(violations, types.Violation{
				RuleID:   hardcodedRuntimeKnobRuleID,
				File:     filepath.ToSlash(scanRoot),
				Message:  fmt.Sprintf("walk runtime knob scan root: %v", walkErr),
				Severity: "error",
			})
		}
	}

	return violations
}

// ValidateHardcodedRuntimeKnobs evaluates one Go file for hardcoded runtime knobs.
func ValidateHardcodedRuntimeKnobs(relPath string, content []byte, identifiers []string) []types.Violation {
	if len(identifiers) == 0 {
		return nil
	}

	fset := token.NewFileSet()
	fileNode, err := parser.ParseFile(fset, relPath, content, 0)
	if err != nil {
		return nil
	}

	lookup := buildKnobLookup(identifiers)
	var violations []types.Violation

	ast.Inspect(fileNode, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.ValueSpec:
			violations = append(violations, checkHardcodedValueSpec(typed, relPath, fset, lookup)...)
		case *ast.AssignStmt:
			violations = append(violations, checkHardcodedAssignStmt(typed, relPath, fset, lookup)...)
		case *ast.KeyValueExpr:
			violations = append(violations, checkHardcodedKeyValueExpr(typed, relPath, fset, lookup)...)
		}
		return true
	})

	return violations
}

func buildKnobLookup(identifiers []string) map[string]struct{} {
	lookup := make(map[string]struct{}, len(identifiers))
	for _, identifier := range identifiers {
		trimmed := strings.TrimSpace(identifier)
		if trimmed == "" {
			continue
		}
		lookup[trimmed] = struct{}{}
	}

	return lookup
}

func checkHardcodedValueSpec(spec *ast.ValueSpec, relPath string, fset *token.FileSet, lookup map[string]struct{}) []types.Violation {
	var violations []types.Violation
	for index, name := range spec.Names {
		if !matchesRuntimeKnob(name.Name, lookup) {
			continue
		}

		if index >= len(spec.Values) || !containsHardcodedLiteral(spec.Values[index]) {
			continue
		}

		violations = append(violations, newHardcodedRuntimeKnobViolation(relPath, name.Name, fset.Position(name.Pos()).Line))
	}

	return violations
}

func checkHardcodedAssignStmt(stmt *ast.AssignStmt, relPath string, fset *token.FileSet, lookup map[string]struct{}) []types.Violation {
	var violations []types.Violation
	for index, left := range stmt.Lhs {
		name, ok := extractAssignedIdentifier(left)
		if !ok || !matchesRuntimeKnob(name, lookup) {
			continue
		}

		if index >= len(stmt.Rhs) || !containsHardcodedLiteral(stmt.Rhs[index]) {
			continue
		}

		violations = append(violations, newHardcodedRuntimeKnobViolation(relPath, name, fset.Position(left.Pos()).Line))
	}

	return violations
}

func checkHardcodedKeyValueExpr(expr *ast.KeyValueExpr, relPath string, fset *token.FileSet, lookup map[string]struct{}) []types.Violation {
	keyName := extractKeyName(expr.Key)
	if !matchesRuntimeKnob(keyName, lookup) || !containsHardcodedLiteral(expr.Value) {
		return nil
	}

	return []types.Violation{newHardcodedRuntimeKnobViolation(relPath, keyName, fset.Position(expr.Pos()).Line)}
}

func extractAssignedIdentifier(expr ast.Expr) (string, bool) {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name, true
	case *ast.SelectorExpr:
		return typed.Sel.Name, true
	default:
		return "", false
	}
}

func extractKeyName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		return typed.Sel.Name
	default:
		return ""
	}
}

func matchesRuntimeKnob(name string, lookup map[string]struct{}) bool {
	if name == "" {
		return false
	}

	if _, ok := lookup[name]; ok {
		return true
	}

	for identifier := range lookup {
		if strings.HasSuffix(name, identifier) {
			return true
		}
	}

	return false
}

func containsHardcodedLiteral(expr ast.Expr) bool {
	switch typed := expr.(type) {
	case *ast.BasicLit:
		return true
	case *ast.UnaryExpr:
		return containsHardcodedLiteral(typed.X)
	case *ast.BinaryExpr:
		return containsHardcodedLiteral(typed.X) || containsHardcodedLiteral(typed.Y)
	case *ast.CallExpr:
		return exprListContainsHardcodedLiteral(typed.Args)
	case *ast.CompositeLit:
		return exprListContainsHardcodedLiteral(typed.Elts)
	case *ast.KeyValueExpr:
		return containsHardcodedLiteral(typed.Value)
	case *ast.ParenExpr:
		return containsHardcodedLiteral(typed.X)
	default:
		return false
	}
}

func exprListContainsHardcodedLiteral(exprs []ast.Expr) bool {
	for _, expr := range exprs {
		if containsHardcodedLiteral(expr) {
			return true
		}
	}

	return false
}

func newHardcodedRuntimeKnobViolation(relPath, name string, line int) types.Violation {
	return types.Violation{
		RuleID:   hardcodedRuntimeKnobRuleID,
		File:     relPath,
		Function: name,
		Line:     line,
		Message:  fmt.Sprintf("runtime knob %s is hardcoded; move it to configuration or an injected dependency", name),
		Severity: "warn",
	}
}

func normalizePolicyPrefixes(prefixes []string) []string {
	normalized := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		trimmed := strings.TrimSpace(prefix)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, filepath.ToSlash(trimmed))
	}

	return normalized
}
