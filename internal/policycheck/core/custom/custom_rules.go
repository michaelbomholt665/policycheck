// internal/policycheck/core/custom/custom_rules.go
// Package custom implements user-defined regex-based policy checks.
// It allows for project-specific rules to be defined in the policy configuration.
package custom

import (
	"bufio"
	"context"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/host"
	"policycheck/internal/policycheck/types"
)

type directoryWalker interface {
	WalkDirectoryTree(root string, fn fs.WalkDirFunc) error
}

// CheckCustomRules applies regex patterns to file content based on configuration.
func CheckCustomRules(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation {
	walk, err := host.ResolveWalkProvider()
	if err != nil {
		return nil
	}

	// Ensure our dynamically loaded dependency fulfills interface requirement.
	walker, ok := walk.(directoryWalker)
	if !ok {
		return nil
	}

	var violations []types.Violation

	for _, rule := range cfg.CustomRules {
		if !rule.Enabled {
			continue
		}

		for _, scanRoot := range getScanRoots(root, cfg, rule) {
			violations = append(violations, scanCustomRule(root, scanRoot, rule, walker)...)
		}
	}

	return violations
}

// getScanRoots determines the set of directories to scan for a custom rule.
func getScanRoots(root string, cfg config.PolicyConfig, rule config.PolicyCustomRule) []string {
	if rule.FileGlob == "" && len(cfg.Paths.ProductionRoots) > 0 {
		roots := make([]string, 0, len(cfg.Paths.ProductionRoots))
		for _, pr := range cfg.Paths.ProductionRoots {
			roots = append(roots, filepath.Join(root, pr))
		}
		return roots
	}
	return []string{root}
}

// scanCustomRule performs a regex-based scan of files matching the rule's criteria.
func scanCustomRule(root, scanRoot string, rule config.PolicyCustomRule, walk directoryWalker) []types.Violation {
	var violations []types.Violation
	_ = walk.WalkDirectoryTree(scanRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		rel, _ := filepath.Rel(root, path)

		if !MatchesRuleCriteria(rel, path, rule) {
			return nil
		}

		if content, readErr := host.ReadFile(path); readErr == nil {
			violations = append(violations, CheckContentForPattern(rel, string(content), rule)...)
		}
		return nil
	})
	return violations
}

// MatchesRuleCriteria checks if a file matches the language and glob filters of a rule.
func MatchesRuleCriteria(rel, path string, rule config.PolicyCustomRule) bool {
	// Language filter (by extension)
	if rule.Language != "" && strings.ToLower(rule.Language) != "any" {
		ext := filepath.Ext(path)
		match := false
		lang := strings.ToLower(rule.Language)
		switch lang {
		case "go":
			match = ext == ".go"
		case "python":
			match = ext == ".py"
		case "typescript":
			match = ext == ".ts" || ext == ".tsx"
		default:
			match = strings.EqualFold(ext, "."+rule.Language)
		}
		if !match {
			return false
		}
	}

	// Glob filter
	if rule.FileGlob != "" {
		if matched := matchGlob(rel, rule.FileGlob); !matched {
			return false
		}
	}

	return true
}

// matchGlob is a simple glob matcher that handles **.
func matchGlob(rel, glob string) bool {
	// Simple conversion of glob to regex
	// * -> [^/]*
	// ** -> .*
	// . -> \.
	// / -> /

	pattern := glob
	pattern = strings.ReplaceAll(pattern, ".", "\\.")

	// Temporarily replace ** to avoid * matching it
	pattern = strings.ReplaceAll(pattern, "**", "___DOUBLE_STAR___")
	pattern = strings.ReplaceAll(pattern, "*", "[^/]*")
	pattern = strings.ReplaceAll(pattern, "___DOUBLE_STAR___", ".*")

	// Handle / as literal
	pattern = "^" + pattern + "$"

	re, err := regexp.Compile(pattern)
	if err != nil {
		// Fallback to filepath.Match if regex conversion fails
		matched, _ := filepath.Match(glob, rel)
		return matched
	}

	return re.MatchString(rel)
}

// CheckContentForPattern scans content line by line for a regex pattern.
func CheckContentForPattern(rel, content string, rule config.PolicyCustomRule) []types.Violation {
	if rule.CompiledPattern == nil {
		return nil
	}

	violations := []types.Violation{}
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 1
	for scanner.Scan() {
		lineText := scanner.Text()
		if rule.CompiledPattern.MatchString(lineText) {
			violations = append(violations, types.Violation{
				RuleID:   "custom." + rule.ID,
				File:     rel,
				Message:  rule.Message,
				Severity: rule.Severity,
				Line:     lineNum,
			})
		}
		lineNum++
	}

	return violations
}
