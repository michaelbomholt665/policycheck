// internal/policycheck/core/topology.go
// Enforces package file count limits and doc.go presence/structure rules.

package core

const ScopeProjectRepo = true


import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"policycheck/internal/policycheck/types"
	"policycheck/internal/policycheck/utils"
)

type pkgStats struct {
	productionGo int
	hasDoc       bool
	docPath      string
	hasConcerns  bool
	concernCount int
	docPrefixOK  bool
}

// CheckPackageTopologyPolicies validates package-level file count limits and doc.go requirements.
func CheckPackageTopologyPolicies(root string) []types.Violation {
	stats, issues := collectPackageStats(root)
	violations := validatePackageStats(stats)
	return append(issues, violations...)
}

// collectPackageStats gathers package-level statistics for all packages in the project.
func collectPackageStats(root string) (map[string]*pkgStats, []types.Violation) {
	stats := map[string]*pkgStats{}
	issues := []types.Violation{}
	bases := []string{
		filepath.Join(root, "cmd"),
		filepath.Join(root, "internal"),
		filepath.Join(root, "test"),
	}

	for _, base := range bases {
		_ = filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
			if violation, skip := buildTopologyWalkViolation(base, path, walkErr); skip {
				if violation.Message != "" {
					issues = append(issues, violation)
				}
				return nil
			}
			if shouldSkipTopologyEntry(path, entry) {
				return nil
			}
			if isGeneratedGoFile(path, entry.Name()) {
				return nil
			}
			processTopologyFile(root, path, entry.Name(), stats, &issues)
			return nil
		})
	}
	return stats, issues
}

// buildTopologyWalkViolation converts a WalkDir error into a policy violation when appropriate.
func buildTopologyWalkViolation(base, path string, walkErr error) (types.Violation, bool) {
	if walkErr == nil {
		return types.Violation{}, false
	}
	if os.IsNotExist(walkErr) && path == base {
		return types.Violation{}, true
	}
	return types.Violation{
		Path:    path,
		Message: fmt.Sprintf("error walking directory: %v", walkErr),
	}, true
}

// shouldSkipTopologyEntry reports whether a directory walk entry is irrelevant to package topology checks.
func shouldSkipTopologyEntry(path string, entry fs.DirEntry) bool {
	return entry.IsDir() || filepath.Ext(path) != ".go"
}

// processTopologyFile updates package statistics for a single Go file.
func processTopologyFile(
	root string,
	path string,
	name string,
	stats map[string]*pkgStats,
	issues *[]types.Violation,
) {
	dir := utils.RelOrAbs(root, filepath.Dir(path))
	st := ensurePkgStats(stats, dir)

	if name == "doc.go" {
		populateDocStats(st, path, dir, issues)
		return
	}
	if strings.HasSuffix(name, "_test.go") {
		return
	}
	st.productionGo++
}

// ensurePkgStats returns the package stats for a directory, creating them on first use.
func ensurePkgStats(stats map[string]*pkgStats, dir string) *pkgStats {
	st := stats[dir]
	if st != nil {
		return st
	}
	st = &pkgStats{}
	stats[dir] = st
	return st
}

// populateDocStats reads a doc.go file and populates package documentation statistics.
func populateDocStats(st *pkgStats, path, dir string, issues *[]types.Violation) {
	st.hasDoc = true
	st.docPath = dir + "/doc.go"
	st.docPrefixOK = true

	contentBytes, err := os.ReadFile(path)
	if err != nil {
		*issues = append(*issues, types.Violation{Path: path, Message: fmt.Sprintf("unable to read doc.go: %v", err)})
		return
	}
	content := string(contentBytes)
	st.hasConcerns, st.concernCount = parseDocGoConcerns(content)
	if strings.HasPrefix(content, "// Package ") {
		firstLine := strings.Split(content, "\n")[0]
		parts := strings.SplitN(firstLine, " ", 4)
		if len(parts) >= 4 {
			desc := parts[3]
			if len(desc) == 0 || !(desc[0] >= 'A' && desc[0] <= 'Z') {
				st.docPrefixOK = false
			}
		} else {
			st.docPrefixOK = false
		}
	} else {
		st.docPrefixOK = false
	}
}

// isGeneratedGoFile determines if a Go source file is generated code.
func isGeneratedGoFile(path, name string) bool {
	if strings.HasPrefix(name, "zz_generated") || strings.HasSuffix(name, ".gen.go") || strings.HasSuffix(name, ".pb.go") {
		return true
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	header := string(content)
	if len(header) > 512 {
		header = header[:512]
	}
	return strings.Contains(header, "Code generated") && strings.Contains(header, "DO NOT EDIT")
}

// validatePackageStats validates all collected package statistics for policy violations.
func validatePackageStats(stats map[string]*pkgStats) []types.Violation {
	violations := []types.Violation{}
	dirs := make([]string, 0, len(stats))
	for dir := range stats {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	for _, dir := range dirs {
		if strings.HasPrefix(filepath.ToSlash(dir), "cmd/policycheck") {
			continue
		}
		st := stats[dir]
		if st.productionGo == 0 {
			continue
		}
		violations = append(violations, checkPkgFileCount(dir, st)...)
		violations = append(violations, checkPkgDocGo(dir, st)...)
	}
	return violations
}

// checkPkgFileCount validates that a package does not exceed the maximum file count.
func checkPkgFileCount(dir string, st *pkgStats) []types.Violation {
	if st.productionGo > 10 {
		return []types.Violation{{
			Path:    dir,
			Message: fmt.Sprintf("package has %d production .go files; max is 10 + doc.go (split into sub-packages)", st.productionGo),
		}}
	}
	return nil
}

// checkPkgDocGo validates that a package has a proper doc.go file with required sections.
func checkPkgDocGo(dir string, st *pkgStats) []types.Violation {
	if !st.hasDoc {
		return []types.Violation{{Path: dir, Message: "missing required doc.go package documentation file"}}
	}
	if !st.docPrefixOK {
		return []types.Violation{{Path: st.docPath, Message: "doc.go must start with `// Package <name> <Description>` with capital first letter of Description"}}
	}
	if !st.hasConcerns {
		return []types.Violation{{Path: st.docPath, Message: "doc.go must contain `Package Concerns:` section with concern bullets"}}
	}
	if st.concernCount == 0 {
		return []types.Violation{{Path: st.docPath, Message: "doc.go `Package Concerns:` section must include at least 1 bullet"}}
	}
	if st.concernCount > 2 {
		return []types.Violation{{Path: st.docPath, Message: fmt.Sprintf("doc.go declares %d package concerns; max is 2", st.concernCount)}}
	}
	return nil
}

// parseDocGoConcerns extracts presence and count of "Package Concerns:" bullets from a doc.go file.
func parseDocGoConcerns(content string) (bool, int) {
	lines := strings.Split(content, "\n")
	inSection := false
	hasSection := false
	count := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		bare := strings.TrimSpace(strings.TrimPrefix(trimmed, "//"))
		lower := strings.ToLower(bare)

		if lower == "package concerns:" {
			hasSection = true
			inSection = true
			continue
		}
		if !inSection {
			continue
		}
		if strings.HasPrefix(bare, "#") {
			break
		}
		if bare == "" || trimmed == "//" {
			continue
		}
		if strings.HasPrefix(bare, "- ") {
			count++
			continue
		}
		break
	}
	return hasSection, count
}
