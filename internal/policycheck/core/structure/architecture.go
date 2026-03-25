// internal/policycheck/core/structure/architecture.go
package structure

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/host"
	"policycheck/internal/policycheck/types"
)

// CheckArchitecture validates configured directory structure rules.
func CheckArchitecture(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation {
	if !cfg.Architecture.Enforce {
		return nil
	}

	violations := []types.Violation{}
	for _, rule := range cfg.Architecture.Roots {
		violations = append(violations, checkArchitectureRoot(root, rule)...)
	}

	violations = append(violations, checkImportDirectionality(root)...)

	return violations
}

func checkImportDirectionality(root string) []types.Violation {
	walk, err := host.ResolveWalkProvider()
	if err != nil {
		return nil
	}

	var violations []types.Violation
	internalBase := filepath.Join(root, "internal")

	_ = walk.WalkDirectoryTree(internalBase, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		content, err := host.ReadFile(path)
		if err != nil {
			return nil
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, string(content), parser.ImportsOnly)
		if err != nil {
			return nil
		}

		rel, _ := filepath.Rel(root, path)

		for _, imp := range f.Imports {
			if imp.Path != nil {
				importPath := strings.Trim(imp.Path.Value, `"`)
				// Simple check to ensure we aren't importing anything under 'cmd/' in the same module.
				// This assumes module imports look like "module-name/cmd/..." or relative imports.
				// Given we're inside policycheck: "policycheck/cmd/..."
				if strings.Contains(importPath, "/cmd/") || strings.HasPrefix(importPath, "cmd/") {
					// Make sure it's actually the local cmd directory. We will just ban any import with /cmd/ if it shares the prefix of this module, but since we don't have the module name easily, any import containing "/cmd/" or "cmd/" is suspicious, but let's be more precise.
					// Let's assume all internal module imports start with the module name or similar.
					// For a simple generic rule: packages in internal/ shouldn't import from cmd/ at all, so we check if it contains the project's cmd path.
					// If importPath contains "cmd/", we'll flag it.
					violations = append(violations, types.Violation{
						RuleID:   "structure.architecture",
						File:     filepath.ToSlash(rel),
						Line:     fset.Position(imp.Pos()).Line,
						Message:  fmt.Sprintf("package inside internal/ imports %q, which is inside cmd/", importPath),
						Severity: "error",
					})
				}
			}
		}
		return nil
	})

	return violations
}

func checkArchitectureRoot(root string, rule config.PolicyArchitectureRoot) []types.Violation {
	base := filepath.Join(root, filepath.FromSlash(rule.Path))
	entries, err := os.ReadDir(base)
	if err != nil {
		return []types.Violation{BuildArchitectureReadViolation(rule.Path, err)}
	}

	allowed := BuildNameSet(rule.AllowedChildren)
	ignored := BuildNameSet(rule.IgnoreChildren)
	violations := []types.Violation{}
	for _, entry := range entries {
		if viol, ok := ValidateArchitectureEntry(rule.Path, entry.Name(), entry.IsDir(), allowed, ignored, rule.AllowedChildren); ok {
			violations = append(violations, viol)
		}
	}
	return violations
}

// BuildArchitectureReadViolation converts a directory read error into a policy violation.
func BuildArchitectureReadViolation(rulePath string, err error) types.Violation {
	message := fmt.Sprintf("unable to read architecture root: %v", err)
	if os.IsNotExist(err) {
		message = "configured architecture root does not exist"
	}
	return types.Violation{
		RuleID:   "structure.architecture",
		File:     filepath.ToSlash(rulePath),
		Message:  message,
		Severity: "error",
	}
}

// BuildNameSet creates a lookup set for configured names.
func BuildNameSet(names []string) map[string]struct{} {
	items := make(map[string]struct{}, len(names))
	for _, name := range names {
		items[name] = struct{}{}
	}
	return items
}

// ValidateArchitectureEntry validates a single top-level directory entry against a root rule.
func ValidateArchitectureEntry(
	rulePath, name string,
	isDir bool,
	allowed map[string]struct{},
	ignored map[string]struct{},
	allowedList []string,
) (types.Violation, bool) {
	if !isDir {
		return types.Violation{}, false
	}

	if _, ok := ignored[name]; ok {
		return types.Violation{}, false
	}
	if _, ok := allowed[name]; ok {
		return types.Violation{}, false
	}

	return types.Violation{
		RuleID:   "structure.architecture",
		File:     filepath.ToSlash(filepath.Join(rulePath, name)),
		Message:  fmt.Sprintf("top-level directory is not allowed under %s; allowed children: %s", filepath.ToSlash(rulePath), strings.Join(allowedList, ", ")),
		Severity: "error",
	}, true
}
