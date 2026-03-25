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

	violations = append(violations, checkArchitectureConcerns(root, cfg.Architecture.Concerns)...)
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

		rel, _ := filepath.Rel(root, path)
		if viols := checkFileImportDirectionality(path, rel); viols != nil {
			violations = append(violations, viols...)
		}
		return nil
	})

	return violations
}

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

func checkArchitectureRoot(root string, rule config.PolicyArchitectureRoot) []types.Violation {
	base := filepath.Join(root, filepath.FromSlash(rule.Path))
	entries, err := os.ReadDir(base)
	if err != nil {
		return []types.Violation{BuildArchitectureReadViolation(rule.Path, err)}
	}

	allowed := BuildNameSet(rule.AllowedChildren)
	ignored := BuildNameSet(rule.IgnoreChildren)
	violations := []types.Violation{}
	entryRule := architectureEntryRule{
		rulePath:    rule.Path,
		allowed:     allowed,
		ignored:     ignored,
		allowedList: rule.AllowedChildren,
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

type architectureEntryRule struct {
	rulePath    string
	allowed     map[string]struct{}
	ignored     map[string]struct{}
	allowedList []string
}

// ValidateArchitectureEntry validates a single top-level directory entry against a root rule.
func ValidateArchitectureEntry(
	rulePath, name string,
	isDir bool,
	allowed map[string]struct{},
	ignored map[string]struct{},
	allowedList []string,
) (types.Violation, bool) {
	return validateArchitectureEntry(architectureEntryRule{
		rulePath:    rulePath,
		allowed:     allowed,
		ignored:     ignored,
		allowedList: allowedList,
	}, name, isDir)
}

func validateArchitectureEntry(rule architectureEntryRule, name string, isDir bool) (types.Violation, bool) {
	if !isDir {
		return types.Violation{}, false
	}

	if _, ok := rule.ignored[name]; ok {
		return types.Violation{}, false
	}
	if _, ok := rule.allowed[name]; ok {
		return types.Violation{}, false
	}

	return types.Violation{
		RuleID:   "structure.architecture",
		File:     filepath.ToSlash(filepath.Join(rule.rulePath, name)),
		Message:  fmt.Sprintf("top-level directory is not allowed under %s; allowed children: %s", filepath.ToSlash(rule.rulePath), strings.Join(rule.allowedList, ", ")),
		Severity: "error",
	}, true
}

func checkArchitectureConcerns(root string, concerns []config.PolicyArchitectureTopic) []types.Violation {
	var violations []types.Violation

	for _, concern := range concerns {
		violations = append(violations, validateArchitectureConcern(root, concern)...)
	}

	return violations
}

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
