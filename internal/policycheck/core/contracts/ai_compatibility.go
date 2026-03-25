// internal/policycheck/core/contracts/ai_compatibility.go
package contracts

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/host"
	"policycheck/internal/policycheck/types"
)

// CheckAICompatibility evaluates the AI compatibility policy for the repository.
func CheckAICompatibility(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation {
	var viols []types.Violation
	walkProvider, err := host.ResolveWalkProvider()
	if err != nil {
		return []types.Violation{{RuleID: "ai-compatibility", Message: fmt.Sprintf("checkAICompatibility: %v", err), Severity: "error"}}
	}

	foundFlags := false
	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}

		relPath, _ := filepath.Rel(root, path)
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		isWrapper, v, _ := ValidateAICompatibility(relPath, string(content), cfg.AICompatibility.RequiredFlags)
		if isWrapper {
			return nil
		}

		if len(v) == 0 {
			foundFlags = true
		} else {
			viols = append(viols, v...)
		}
		return nil
	}

	if err = walkProvider.WalkDirectoryTree(root, walkFn); err != nil {
		return []types.Violation{{RuleID: "ai-compatibility", Message: fmt.Sprintf("checkAICompatibility: %v", err), Severity: "error"}}
	}

	if foundFlags {
		return nil
	}

	if len(viols) == 0 {
		return []types.Violation{{RuleID: "ai-compatibility", Message: "no AI compatibility flags found in any file", Severity: "error"}}
	}

	return viols
}

// ValidateAICompatibility evaluates the content for AI compatibility flags.
func ValidateAICompatibility(relPath string, content string, requiredFlags []string) (bool, []types.Violation, error) {
	if strings.Contains(content, "RunCLI(") && !strings.Contains(content, "flag.") && !strings.Contains(content, "cobra.") {
		// Thin wrapper heuristic
		return true, nil, nil
	}

	var missing []string
	for _, req := range requiredFlags {
		if !strings.Contains(content, req) {
			missing = append(missing, req)
		}
	}

	if len(missing) > 0 {
		return false, []types.Violation{{
			RuleID:   "ai-compatibility",
			File:     relPath,
			Message:  fmt.Sprintf("missing required AI compatibility flags: %v", missing),
			Severity: "error",
		}}, nil
	}

	return false, nil, nil
}
