// internal/policycheck/core/contracts/cli_formatter.go
// Package contracts/cli_formatter enforces the use of audience-aware output stylers.
// It flags direct use of fmt.Print* in CLI command implementation files to ensure consistent UI.
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

// CheckCLIFormatter evaluates the CLI formatting policies for the repository.
func CheckCLIFormatter(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation {
	var viols []types.Violation

	for _, relPath := range cfg.CLIFormatter.RequiredFiles {
		absPath := filepath.Join(root, relPath)
		content, err := os.ReadFile(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			viols = append(viols, types.Violation{
				RuleID:   "cli-formatter",
				File:     relPath,
				Message:  fmt.Sprintf("checkCLIFormatter: %v", err),
				Severity: "error",
			})
			continue
		}

		v, err := ValidateCLIFormatter(relPath, string(content))
		if err != nil {
			viols = append(viols, types.Violation{
				RuleID:   "cli-formatter",
				File:     relPath,
				Message:  fmt.Sprintf("checkCLIFormatter: %v", err),
				Severity: "error",
			})
			continue
		}
		viols = append(viols, v...)
	}

	return viols
}

// ValidateCLIFormatter evaluates the content of a file for forbidden CLI output patterns.
func ValidateCLIFormatter(relPath string, content string) ([]types.Violation, error) {
	var viols []types.Violation
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Very simple detection of forbidden fmt.* prints.
		// A full implementation would use AST, but string matching is consistent with the design doc's pure string checking phase.
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		if strings.Contains(line, "fmt.Print(") || strings.Contains(line, "fmt.Printf(") || strings.Contains(line, "fmt.Println(") {
			viols = append(viols, types.Violation{
				RuleID:   "cli-formatter",
				File:     relPath,
				Line:     i + 1,
				Message:  "direct stdout via fmt.Print* is forbidden in CLI code; use audience-aware formatters",
				Severity: "error",
			})
		}
	}

	return viols, nil
}
