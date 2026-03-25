// internal/policycheck/core/contracts/go_version.go
package contracts

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/types"
)

// CheckGoVersion evaluates the go version policy for the repository.
func CheckGoVersion(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation {
	modPath := filepath.Join(root, "go.mod")
	content, err := os.ReadFile(modPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []types.Violation{{
				RuleID:   "go-version",
				File:     "go.mod",
				Message:  "missing go.mod file",
				Severity: "error",
			}}
		}
		// Return contextual error as violation if we cannot read it
		return []types.Violation{{
			RuleID:   "go-version",
			File:     "go.mod",
			Message:  fmt.Sprintf("checkGoVersion: %v", err),
			Severity: "error",
		}}
	}

	viols, goVersion, err := ValidateGoVersion(string(content), cfg.GoVersion.AllowedPrefixes)
	if err != nil {
		return []types.Violation{{
			RuleID:   "go-version",
			File:     "go.mod",
			Message:  fmt.Sprintf("checkGoVersion: %v", err),
			Severity: "error",
		}}
	}

	if goVersion != "" {
		viols = append(viols, scanForVersionMismatches(root, goVersion)...)
	}

	return viols
}

func scanForVersionMismatches(root, goVersion string) []types.Violation {
	var viols []types.Violation

	// Files to check: Dockerfile, .github/workflows/*.yml, etc.
	filesToCheck := []string{
		"Dockerfile",
		".github/workflows/ci.yml",
		".github/workflows/test.yml",
		".gitlab-ci.yml",
	}

	for _, file := range filesToCheck {
		path := filepath.Join(root, filepath.FromSlash(file))
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		
		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			// Naive check: if line contains "go " or "golang" or "GO_VERSION", it might be declaring a version
			if strings.Contains(strings.ToLower(line), goVersion) {
				continue
			}

			// Specifically flag mismatched explicit version strings
			// A real implementation might use a regex to find semantic versions and compare
			if strings.Contains(line, "1.2") && !strings.Contains(line, goVersion) {
				if strings.Contains(line, "GO_VERSION") || strings.Contains(line, "go:") || strings.Contains(line, "golang:") {
					viols = append(viols, types.Violation{
						RuleID:   "go-version-mismatch",
						File:     filepath.ToSlash(file),
						Line:     i + 1,
						Message:  fmt.Sprintf("potential go version mismatch; go.mod is %s but found: %s", goVersion, strings.TrimSpace(line)),
						Severity: "warn",
					})
				}
			}
		}
	}
	return viols
}

// ValidateGoVersion evaluates the go.mod content against allowed prefixes.
func ValidateGoVersion(content string, allowedPrefixes []string) ([]types.Violation, string, error) {
	lines := strings.Split(content, "\n")
	var goVersion string
	var toolchain string
	var viols []types.Violation

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "go ") {
			goVersion = strings.TrimPrefix(line, "go ")
		}
		if strings.HasPrefix(line, "toolchain ") {
			toolchain = strings.TrimPrefix(line, "toolchain ")
		}
	}

	if goVersion == "" {
		viols = append(viols, types.Violation{
			RuleID:   "go-version",
			File:     "go.mod",
			Message:  "missing 'go' directive in go.mod",
			Severity: "error",
		})
		return viols, "", nil
	}

	if toolchain == "" {
		viols = append(viols, types.Violation{
			RuleID:   "go-version",
			File:     "go.mod",
			Message:  "missing 'toolchain' directive in go.mod",
			Severity: "error",
		})
	}

	validPrefix := false
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(goVersion, prefix) {
			validPrefix = true
			break
		}
	}

	if !validPrefix {
		viols = append(viols, types.Violation{
			RuleID:   "go-version",
			File:     "go.mod",
			Message:  "go version " + goVersion + " is not in allowed prefixes",
			Severity: "error",
		})
	}

	return viols, goVersion, nil
}
