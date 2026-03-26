// internal/policycheck/core/structure/test_location.go
// Package structure implements structural consistency policies for Go packages.
// It verifies package-level test locations.
package structure

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/host"
	"policycheck/internal/policycheck/types"
)

// CheckTestLocation ensures all test files are in the configured allowed test directories.
func CheckTestLocation(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation {
	walk, err := host.ResolveWalkProvider()
	if err != nil {
		return nil
	}

	violations := []types.Violation{}
	for _, relBase := range cfg.Paths.TestScanRoots {
		base := filepath.Join(root, filepath.FromSlash(relBase))
		_ = walk.WalkDirectoryTree(base, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}

			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if !HasPrefix(rel, cfg.Paths.AllowedTestPrefixes) {
				violations = append(violations, types.Violation{
					RuleID:   "structure.test_location",
					File:     rel,
					Message:  "test files must be located in internal/tests/ to adhere to testing standards",
					Severity: "error",
				})
			}
			return nil
		})
	}
	return violations
}
