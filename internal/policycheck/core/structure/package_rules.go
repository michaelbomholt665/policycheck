// internal/policycheck/core/structure/package_rules.go
// Package structure implements structural consistency policies for Go packages.
// It verifies package-level documentation, file counts, and architecture concerns.
package structure

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/host"
	"policycheck/internal/policycheck/types"
	"policycheck/internal/policycheck/utils"
)

// PackageStats holds structural statistics for a single Go package.
type PackageStats struct {
	ProductionGo int
	HasDoc       bool
	DocPath      string
	HasConcerns  bool
	ConcernCount int
	DocPrefixOK  bool
}

// CheckPackageRules validates package-level file limits and doc.go requirements.
func CheckPackageRules(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation {
	walk, err := host.ResolveWalkProvider()
	if err != nil {
		return nil
	}

	stats := make(map[string]*PackageStats)
	walkFn := createWalkFn(root, stats, cfg.PackageRules)

	for _, scanRoot := range cfg.PackageRules.ScanRoots {
		base := filepath.Join(root, filepath.FromSlash(scanRoot))
		_ = walk.WalkDirectoryTree(base, walkFn)
	}

	return ValidatePackageStats(stats, cfg.PackageRules)
}

// createWalkFn returns a fs.WalkDirFunc that collects package statistics.
func createWalkFn(root string, stats map[string]*PackageStats, cfg config.PolicyPackageRulesConfig) fs.WalkDirFunc {
	return func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}

		content, _ := host.ReadFile(path)
		if utils.IsGeneratedFile(content) {
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		dir := filepath.ToSlash(filepath.Dir(rel))

		if isPackageRulesExcluded(dir, cfg.ExcludePrefixes) {
			return nil
		}

		st, ok := stats[dir]
		if !ok {
			st = &PackageStats{}
			stats[dir] = st
		}

		if d.Name() == "doc.go" {
			st.HasDoc = true
			st.DocPath = rel
			populateDocStats(st, path)
		} else if !strings.HasSuffix(d.Name(), "_test.go") {
			st.ProductionGo++
		}

		return nil
	}
}

// isPackageRulesExcluded determines whether a directory should be skipped.
func isPackageRulesExcluded(dir string, prefixes []string) bool {
	if strings.HasPrefix(dir, "cmd/policycheck") || strings.HasPrefix(dir, "internal/policycheck") {
		return true
	}

	return host.HasPrefix(dir, prefixes)
}

// populateDocStats reads the file content and evaluates the doc.go requirements.
func populateDocStats(st *PackageStats, path string) {
	contentBytes, err := host.ReadFile(path)
	if err != nil {
		return
	}
	content := string(contentBytes)
	st.HasConcerns, st.ConcernCount = ParseDocGoConcerns(content)
	st.DocPrefixOK = checkDocPrefix(content)
}

// checkDocPrefix verifies that the doc.go file starts with the required Package comment.
func checkDocPrefix(content string) bool {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "package ") {
			break
		}
		if strings.HasPrefix(line, "// Package ") {
			parts := strings.SplitN(line, " ", 4)
			if len(parts) >= 4 {
				desc := parts[3]
				if len(desc) > 0 && desc[0] >= 'A' && desc[0] <= 'Z' {
					return true
				}
			}
			break
		}
	}
	return false
}

// ParseDocGoConcerns extracts presence and count of "Package Concerns:" bullets from a doc.go file.
func ParseDocGoConcerns(content string) (bool, int) {
	lines := strings.Split(content, "\n")
	hasSection := false
	count := 0
	inSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		bare := strings.TrimPrefix(trimmed, "//")
		bare = strings.TrimSpace(bare)
		lower := strings.ToLower(bare)

		if strings.Contains(lower, "package concerns:") {
			hasSection = true
			inSection = true
			continue
		}
		if inSection {
			if strings.HasPrefix(bare, "- ") {
				count++
			} else if bare == "" || !strings.HasPrefix(trimmed, "//") {
				// Section ends at next empty content or non-comment line
				inSection = false
			}
		}
	}
	return hasSection, count
}

// ValidatePackageStats validates all collected package statistics for policy violations.
func ValidatePackageStats(stats map[string]*PackageStats, cfg config.PolicyPackageRulesConfig) []types.Violation {
	violations := []types.Violation{}
	dirs := make([]string, 0, len(stats))
	for dir := range stats {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	for _, dir := range dirs {
		st := stats[dir]
		violations = append(violations, validateSinglePackage(dir, st, cfg)...)
	}
	return violations
}

// validateSinglePackage evaluates the statistics of a single package against policy thresholds.
func validateSinglePackage(dir string, st *PackageStats, cfg config.PolicyPackageRulesConfig) []types.Violation {
	if st.ProductionGo == 0 {
		return nil
	}

	var violations []types.Violation
	if st.ProductionGo == 1 {
		violations = append(violations, types.Violation{
			RuleID:   "structure.package_rules",
			File:     dir,
			Message:  "package contains exactly 1 non-doc file; consider consolidating with another package",
			Severity: "warn",
		})
	}

	if st.ProductionGo > cfg.MaxProductionFiles {
		violations = append(violations, types.Violation{
			RuleID:   "structure.package_rules",
			File:     dir,
			Message:  fmt.Sprintf("package has %d production .go files; max is %d + doc.go (split into sub-packages)", st.ProductionGo, cfg.MaxProductionFiles),
			Severity: "error",
		})
	}

	if !st.HasDoc {
		return append(violations, types.Violation{
			RuleID:   "structure.package_rules",
			File:     dir,
			Message:  "missing required doc.go package documentation file",
			Severity: "error",
		})
	}

	return append(violations, validatePackageDocumentation(st, cfg)...)
}

// validatePackageDocumentation checks doc.go prefix and concerns.
func validatePackageDocumentation(st *PackageStats, cfg config.PolicyPackageRulesConfig) []types.Violation {
	var violations []types.Violation
	if !st.DocPrefixOK {
		violations = append(violations, types.Violation{
			RuleID:   "structure.package_rules",
			File:     st.DocPath,
			Message:  "doc.go must start with `// Package <name> <Description>` with capital first letter of Description",
			Severity: "error",
		})
	}

	if !st.HasConcerns {
		violations = append(violations, types.Violation{
			RuleID:   "structure.package_rules",
			File:     st.DocPath,
			Message:  "doc.go must contain `Package Concerns:` section with concern bullets",
			Severity: "error",
		})
	} else if st.ConcernCount < cfg.MinConcerns {
		violations = append(violations, types.Violation{
			RuleID:   "structure.package_rules",
			File:     st.DocPath,
			Message:  fmt.Sprintf("doc.go `Package Concerns:` section must include at least %d bullet", cfg.MinConcerns),
			Severity: "error",
		})
	} else if st.ConcernCount > cfg.MaxConcerns {
		violations = append(violations, types.Violation{
			RuleID:   "structure.package_rules",
			File:     st.DocPath,
			Message:  fmt.Sprintf("doc.go declares %d package concerns; max is %d", st.ConcernCount, cfg.MaxConcerns),
			Severity: "error",
		})
	}

	return violations
}
