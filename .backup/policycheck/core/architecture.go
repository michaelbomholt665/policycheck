// internal/policycheck/core/architecture.go
// Validates directory structure rules and provides architecture concern printing.

package core

const ScopeProjectRepo = true


import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/types"
)

// CheckArchitecturePolicies validates configured directory structure rules.
func CheckArchitecturePolicies(root string, cfg config.PolicyConfig) []types.Violation {
	if !cfg.Architecture.Enforce {
		return nil
	}

	violations := []types.Violation{}
	for _, rule := range cfg.Architecture.Roots {
		violations = append(violations, checkArchitectureRoot(root, rule)...)
	}
	return violations
}

// checkArchitectureRoot validates a single configured architecture root.
func checkArchitectureRoot(root string, rule config.PolicyArchitectureRoot) []types.Violation {
	entries, err := readArchitectureEntries(root, rule.Path)
	if err != nil {
		return []types.Violation{buildArchitectureReadViolation(rule.Path, err)}
	}

	allowed := buildNameSet(rule.AllowedChildren)
	ignored := buildNameSet(rule.IgnoreChildren)
	violations := make([]types.Violation, 0)
	for _, entry := range entries {
		if violation, ok := validateArchitectureEntry(rule, entry, allowed, ignored); ok {
			violations = append(violations, violation)
		}
	}
	return violations
}

// readArchitectureEntries loads the directory entries for a configured architecture root.
func readArchitectureEntries(root, rulePath string) ([]os.DirEntry, error) {
	base := filepath.Join(root, filepath.FromSlash(rulePath))
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, fmt.Errorf("read architecture root %s: %w", filepath.ToSlash(rulePath), err)
	}
	return entries, nil
}

// buildArchitectureReadViolation converts a directory read error into a policy violation.
func buildArchitectureReadViolation(rulePath string, err error) types.Violation {
	message := fmt.Sprintf("unable to read architecture root: %v", err)
	if os.IsNotExist(err) {
		message = "configured architecture root does not exist"
	}
	return types.Violation{Path: filepath.ToSlash(rulePath), Message: message}
}

// buildNameSet creates a lookup set for configured names.
func buildNameSet(names []string) map[string]struct{} {
	items := make(map[string]struct{}, len(names))
	for _, name := range names {
		items[name] = struct{}{}
	}
	return items
}

// validateArchitectureEntry validates a single top-level directory entry against a root rule.
func validateArchitectureEntry(
	rule config.PolicyArchitectureRoot,
	entry os.DirEntry,
	allowed map[string]struct{},
	ignored map[string]struct{},
) (types.Violation, bool) {
	if !entry.IsDir() {
		return types.Violation{}, false
	}

	name := entry.Name()
	if _, ok := ignored[name]; ok {
		return types.Violation{}, false
	}
	if _, ok := allowed[name]; ok {
		return types.Violation{}, false
	}

	return types.Violation{
		Path: filepath.ToSlash(filepath.Join(rule.Path, name)),
		Message: fmt.Sprintf(
			"top-level directory is not allowed under %s; allowed children: %s",
			filepath.ToSlash(rule.Path),
			strings.Join(rule.AllowedChildren, ", "),
		),
	}, true
}

// PrintConcern searches for a named architecture concern and prints its details to stdout.
func PrintConcern(cfg config.PolicyConfig, concernName string) error {
	for _, concern := range cfg.Architecture.Concerns {
		if !strings.EqualFold(concern.Name, concernName) {
			continue
		}
		printConcernDetails(concern)
		return nil
	}

	available := make([]string, 0, len(cfg.Architecture.Concerns))
	for _, concern := range cfg.Architecture.Concerns {
		available = append(available, concern.Name)
	}
	sort.Strings(available)
	if len(available) == 0 {
		return fmt.Errorf("concern %q not found; no concerns configured", concernName)
	}
	return fmt.Errorf("concern %q not found; available concerns: %s", concernName, strings.Join(available, ", "))
}

// printConcernDetails prints the details of an architecture concern to stdout.
func printConcernDetails(concern config.PolicyArchitectureTopic) {
	fmt.Printf("Concern: %s\n", concern.Name)
	if len(concern.Tags) > 0 {
		fmt.Printf("Tags: %s\n", strings.Join(concern.Tags, ", "))
	}
	printConcernPaths("Roots", concern.Roots)
	printConcernPaths("Config", concern.ConfigPaths)
	printConcernPaths("Schema", concern.SchemaPaths)
	printConcernPaths("Contracts", concern.ContractPaths)
	printConcernPaths("API", concern.APIPaths)
}

// printConcernPaths prints a labeled list of paths to stdout.
func printConcernPaths(label string, paths []string) {
	if len(paths) == 0 {
		return
	}
	fmt.Printf("%s:\n", label)
	for _, p := range paths {
		fmt.Printf("  - %s\n", filepath.ToSlash(p))
	}
}
