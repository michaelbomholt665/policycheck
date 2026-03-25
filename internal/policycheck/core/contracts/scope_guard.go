// internal/policycheck/core/contracts/scope_guard.go
package contracts

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/host"
	"policycheck/internal/policycheck/types"
)

// CheckScopeGuard evaluates the scope guard policy across all files under root.
// It checks for forbidden lifecycle calls like os.WriteFile.
func CheckScopeGuard(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation {
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

		content, readErr := os.ReadFile(path)
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
	var viols []types.Violation
	viols = append(viols, checkForbiddenCalls(relPath, content, cfg)...)
	return viols, nil
}

// checkForbiddenCalls returns a violation for each forbidden lifecycle call
// found in content. This check is always active regardless of other flags.
func checkForbiddenCalls(relPath, content string, cfg config.PolicyConfig) []types.Violation {
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
