// internal/policycheck/core/contracts/scope_guard.go
// Package contracts/scope_guard enforces restrictions on OS-level lifecycle calls.
// It ensures that forbidden calls like os.WriteFile only occur in approved adapter packages.
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

// CheckScopeGuard evaluates the scope guard policy across all files under root.
//
// It checks for forbidden lifecycle calls in scanned source files.
func CheckScopeGuard(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation {
	if !cfg.ScopeGuard.Enabled {
		return nil
	}

	walkProvider, err := host.ResolveWalkProvider()
	if err != nil {
		return scopeGuardWalkError(err)
	}

	supportedExts := map[string]bool{
		".go": true,
		".py": true,
		".ts": true,
		".js": true,
	}

	var viols []types.Violation
	err = walkProvider.WalkDirectoryTree(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		if !supportedExts[ext] {
			return nil
		}

		relPath := toRelSlash(root, path)

		// Hard exclude for internal/router - secondary layer of protection.
		if strings.HasPrefix(relPath, "internal/router/") {
			return nil
		}

		content, readErr := host.ReadFile(path)
		if readErr != nil {
			return nil
		}

		v, _ := ValidateScopeGuard(relPath, string(content), cfg)
		viols = append(viols, v...)
		return nil
	})
	if err != nil {
		return scopeGuardWalkError(err)
	}
	return viols
}

// ValidateScopeGuard checks a single file's content against forbidden lifecycle calls.
func ValidateScopeGuard(relPath string, content string, cfg config.PolicyConfig) ([]types.Violation, error) {
	if !cfg.ScopeGuard.Enabled {
		return nil, nil
	}

	if shouldSkipScopeGuard(relPath, cfg.ScopeGuard) {
		return nil, nil
	}

	var viols []types.Violation
	viols = append(viols, checkForbiddenCalls(relPath, content, cfg)...)
	return viols, nil
}

// shouldSkipScopeGuard determines if a file is exempt from scope guard checks based on its path.
func shouldSkipScopeGuard(relPath string, cfg config.PolicyScopeGuardConfig) bool {
	switch cfg.Mode {
	case config.ScopeGuardModeAllow:
		return true
	case config.ScopeGuardModeRestrict:
		return host.HasPrefix(relPath, cfg.AllowedPathPrefixes)
	default:
		return false
	}
}

// checkForbiddenCalls returns a violation for each forbidden lifecycle call
// found in content. This check is always active regardless of other flags.
func checkForbiddenCalls(relPath, content string, cfg config.PolicyConfig) []types.Violation {
	if filepath.Ext(relPath) == ".go" {
		return checkGoForbiddenCalls(relPath, content, cfg)
	}

	var viols []types.Violation
	for _, call := range cfg.ScopeGuard.ForbiddenCalls {
		if !strings.Contains(content, call) {
			continue
		}
		viols = append(viols, types.Violation{
			RuleID:   "scope-guard",
			File:     relPath,
			Message:  fmt.Sprintf("forbidden lifecycle call found: %s", call),
			Severity: "error",
		})
	}
	return viols
}

// checkGoForbiddenCalls uses AST parsing to find forbidden lifecycle calls in Go source code.
func checkGoForbiddenCalls(relPath, content string, cfg config.PolicyConfig) []types.Violation {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, relPath, content, parser.SkipObjectResolution)
	if err != nil {
		return checkStringForbiddenCalls(relPath, content, cfg)
	}

	seenCalls := make(map[string]struct{})
	ast.Inspect(file, func(node ast.Node) bool {
		callExpr, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		selectorExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		ident, ok := selectorExpr.X.(*ast.Ident)
		if !ok {
			return true
		}

		callName := ident.Name + "." + selectorExpr.Sel.Name
		seenCalls[callName] = struct{}{}
		return true
	})

	var viols []types.Violation
	for _, call := range cfg.ScopeGuard.ForbiddenCalls {
		if _, found := seenCalls[call]; !found {
			continue
		}
		viols = append(viols, types.Violation{
			RuleID:   "scope-guard",
			File:     relPath,
			Message:  fmt.Sprintf("forbidden lifecycle call found: %s", call),
			Severity: "error",
		})
	}

	return viols
}

// checkStringForbiddenCalls fallback uses simple string matching for forbidden calls.
func checkStringForbiddenCalls(relPath, content string, cfg config.PolicyConfig) []types.Violation {
	var viols []types.Violation
	for _, call := range cfg.ScopeGuard.ForbiddenCalls {
		if !strings.Contains(content, call) {
			continue
		}
		viols = append(viols, types.Violation{
			RuleID:   "scope-guard",
			File:     relPath,
			Message:  fmt.Sprintf("forbidden lifecycle call found: %s", call),
			Severity: "error",
		})
	}
	return viols
}

// --- shared helpers ---

// toRelSlash returns the slash-normalised path of target relative to root.
func toRelSlash(root, target string) string {
	rel, _ := filepath.Rel(root, target)
	return filepath.ToSlash(rel)
}

// scopeGuardWalkError wraps a walk-level error into a single-element violation slice.
func scopeGuardWalkError(err error) []types.Violation {
	return []types.Violation{{
		RuleID:   "scope-guard",
		Message:  fmt.Sprintf("checkScopeGuard: %v", err),
		Severity: "error",
	}}
}
