// internal/policycheck/core/structure/architecture.go
// Package structure implements the directory and import relationship validation.
// It enforces rules about allowed children in specific roots and ensures
// that internal packages do not import from the cmd/ directory.
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
//
// It checks both root-level child allowance and cross-module import directionality.
func CheckArchitecture(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation {
	if !cfg.Architecture.Enforce {
		return nil
	}

	violations := []types.Violation{}
	for _, rule := range cfg.Architecture.Roots {
		violations = append(violations, checkArchitectureRoot(root, rule)...)
	}

	violations = append(violations, checkArchitectureConcerns(root, cfg.Architecture.Concerns)...)
	violations = append(violations, checkImportDirectionality(root)...)

	return violations
}

// checkImportDirectionality scans internal directories for forbidden imports.
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

		rel, _ := filepath.Rel(root, path)
		if viols := checkFileImportDirectionality(path, rel); viols != nil {
			violations = append(violations, viols...)
		}
		return nil
	})

	return violations
}

// checkFileImportDirectionality validates that a single file does not import from cmd/.
func checkFileImportDirectionality(path, rel string) []types.Violation {
	content, err := host.ReadFile(path)
	if err != nil {
		return nil
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, string(content), parser.ImportsOnly)
	if err != nil {
		return nil
	}

	var violations []types.Violation
	for _, imp := range f.Imports {
		if imp.Path == nil {
			continue
		}
		importPath := strings.Trim(imp.Path.Value, `"`)
		if strings.Contains(importPath, "/cmd/") || strings.HasPrefix(importPath, "cmd/") {
			violations = append(violations, types.Violation{
				RuleID:   "structure.architecture",
				File:     filepath.ToSlash(rel),
				Line:     fset.Position(imp.Pos()).Line,
				Message:  fmt.Sprintf("package inside internal/ imports %q, which is inside cmd/", importPath),
				Severity: "error",
			})
		}
	}
	return violations
}

// checkArchitectureRoot validates the children of a specific root directory.
func checkArchitectureRoot(root string, rule config.PolicyArchitectureRoot) []types.Violation {
	base := filepath.Join(root, filepath.FromSlash(rule.Path))
	entries, err := os.ReadDir(base)
	if err != nil {
		return []types.Violation{BuildArchitectureReadViolation(rule.Path, err)}
	}

	allowed := BuildNameSet(rule.AllowedChildren)
	ignored := BuildNameSet(rule.IgnoreChildren)
	violations := []types.Violation{}
	entryRule := ArchitectureEntryRule{
		RulePath:    rule.Path,
		Allowed:     allowed,
		Ignored:     ignored,
		AllowedList: rule.AllowedChildren,
	}
	for _, entry := range entries {
		if viol, ok := validateArchitectureEntry(entryRule, entry.Name(), entry.IsDir()); ok {
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

// ArchitectureEntryRule defines the allowance rules for a specific directory path.
type ArchitectureEntryRule struct {
	// RulePath is the relative path from the root being validated.
	RulePath string
	// Allowed is the set of directory names permitted as children.
	Allowed map[string]struct{}
	// Ignored is the set of file or directory names to skip validation for.
	Ignored map[string]struct{}
	// AllowedList is the original slice of allowed names for interpolation in error messages.
	AllowedList []string
}

// ValidateArchitectureEntry validates a single top-level directory entry against a root rule.
func ValidateArchitectureEntry(rule ArchitectureEntryRule, name string, isDir bool) (types.Violation, bool) {
	return validateArchitectureEntry(rule, name, isDir)
}

// validateArchitectureEntry performs the internal check for a directory entry violation.
func validateArchitectureEntry(rule ArchitectureEntryRule, name string, isDir bool) (types.Violation, bool) {
	if !isDir {
		return types.Violation{}, false
	}

	if _, ok := rule.Ignored[name]; ok {
		return types.Violation{}, false
	}
	if _, ok := rule.Allowed[name]; ok {
		return types.Violation{}, false
	}

	return types.Violation{
		RuleID:   "structure.architecture",
		File:     filepath.ToSlash(filepath.Join(rule.RulePath, name)),
		Message:  fmt.Sprintf("top-level directory is not allowed under %s; allowed children: %s", filepath.ToSlash(rule.RulePath), strings.Join(rule.AllowedList, ", ")),
		Severity: "error",
	}, true
}

// checkArchitectureConcerns ensures all defined architectural concerns are valid.
func checkArchitectureConcerns(root string, concerns []config.PolicyArchitectureTopic) []types.Violation {
	var violations []types.Violation

	for _, concern := range concerns {
		violations = append(violations, validateArchitectureConcern(root, concern)...)
	}

	return violations
}

// validateArchitectureConcern validates a single architectural concern and its required properties.
func validateArchitectureConcern(root string, concern config.PolicyArchitectureTopic) []types.Violation {
	var violations []types.Violation

	if strings.TrimSpace(concern.Name) == "" {
		violations = append(violations, types.Violation{
			RuleID:   "structure.architecture",
			File:     "policy-gate.toml",
			Message:  "architecture concern is missing a name",
			Severity: "error",
		})
	}

	if len(concern.Tags) == 0 {
		violations = append(violations, types.Violation{
			RuleID:   "structure.architecture",
			File:     "policy-gate.toml",
			Message:  fmt.Sprintf("architecture concern %q must declare at least one tag", concern.Name),
			Severity: "error",
		})
	}

	violations = append(violations, validateConcernPaths(root, concern.Name, "roots", concern.Roots)...)
	violations = append(violations, validateConcernPaths(root, concern.Name, "config_paths", concern.ConfigPaths)...)
	violations = append(violations, validateConcernPaths(root, concern.Name, "schema_paths", concern.SchemaPaths)...)
	violations = append(violations, validateConcernPaths(root, concern.Name, "contract_paths", concern.ContractPaths)...)
	violations = append(violations, validateConcernPaths(root, concern.Name, "api_paths", concern.APIPaths)...)

	return violations
}

// validateConcernPaths verifies that all paths referenced in a concern exist on disk.
func validateConcernPaths(root, concernName, fieldName string, paths []string) []types.Violation {
	var violations []types.Violation

	for _, relPath := range paths {
		absPath := filepath.Join(root, filepath.FromSlash(relPath))
		if _, err := os.Stat(absPath); err == nil {
			continue
		} else if os.IsNotExist(err) {
			violations = append(violations, types.Violation{
				RuleID:   "structure.architecture",
				File:     filepath.ToSlash(relPath),
				Message:  fmt.Sprintf("architecture concern %q references missing %s entry %q", concernName, fieldName, relPath),
				Severity: "error",
			})
		} else {
			violations = append(violations, types.Violation{
				RuleID:   "structure.architecture",
				File:     filepath.ToSlash(relPath),
				Message:  fmt.Sprintf("architecture concern %q could not read %s entry %q: %v", concernName, fieldName, relPath, err),
				Severity: "error",
			})
		}
	}

	return violations
}
