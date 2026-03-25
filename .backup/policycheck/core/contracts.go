// internal/policycheck/core/contracts.go
// Validates command-line output contracts, AI compatibility patterns, and scope boundaries.

package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/types"
	"policycheck/internal/policycheck/utils"
)

var audienceModePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(^|[^[:alnum:]_])--ai([^[:alnum:]_]|$)`),
	regexp.MustCompile(`(^|[^[:alnum:]_])--user([^[:alnum:]_]|$)`),
}

// CheckAIOutputContractPolicies validates that the command surface supports required AI/user output modes.
func CheckAIOutputContractPolicies(root string, cfg config.PolicyConfig) []types.Violation {
	violations := []types.Violation{}
	violations = append(violations, checkAIAudienceContract(root, cfg)...)
	violations = append(violations, checkCLIConfigAIFormat(root, cfg)...)
	return violations
}

// CheckIsrScopeBoundary verifies that the isr command surface defines explicit scope constants.
func CheckIsrScopeBoundary(root string, cfg config.PolicyConfig) []types.Violation {
	violations := []types.Violation{}
	violations = append(violations, checkScopeProjectRepoBoundary(root, cfg)...)
	violations = append(violations, checkLifecycleDocsBoundary(root, cfg)...)
	return violations
}

// checkAIAudienceContract validates the required audience mode surface.
func checkAIAudienceContract(root string, cfg config.PolicyConfig) []types.Violation {
	target, ok := resolveAIAudienceContractTarget(root, cfg)
	if !ok {
		return nil
	}

	text, err := readContractTarget(root, target)
	if err != nil || hasAudienceModeSupport(text) {
		return nil
	}
	return []types.Violation{{Path: target, Message: "missing required --ai/--user output mode support"}}
}

// checkCLIConfigAIFormat validates that CLI output configuration supports AI mode.
func checkCLIConfigAIFormat(root string, cfg config.PolicyConfig) []types.Violation {
	target, ok := resolveContractTarget(root, cfg, "cli_config")
	if !ok {
		return nil
	}

	text, err := readContractTarget(root, target)
	if err != nil || supportsCLIConfigAIFormat(text) {
		return nil
	}
	return []types.Violation{{Path: target, Message: "CLI output config must support AI format"}}
}

// checkScopeProjectRepoBoundary validates the app scope constant contract.
func checkScopeProjectRepoBoundary(root string, cfg config.PolicyConfig) []types.Violation {
	target, ok := resolveContractTarget(root, cfg, "app_run", "internal/app/run.go")
	if !ok {
		return nil
	}

	text, err := readContractTarget(root, target)
	if err != nil || strings.Contains(text, "ScopeProjectRepo") {
		return nil
	}
	return []types.Violation{{
		Path:    target,
		Message: fmt.Sprintf("isr scope contract: ScopeProjectRepo constant must be defined in %s", target),
	}}
}

// checkLifecycleDocsBoundary validates that retrieval-only code does not mutate files.
func checkLifecycleDocsBoundary(root string, cfg config.PolicyConfig) []types.Violation {
	target, ok := resolveContractTarget(root, cfg, "lifecycle_docs")
	if !ok {
		return nil
	}

	text, err := readContractTarget(root, target)
	if err != nil || !violatesLifecycleDocsBoundary(text) {
		return nil
	}
	return []types.Violation{{
		Path:    target,
		Message: "retrieval boundary violation: lifecycle_docs.go must not call os.WriteFile or os.Rename",
	}}
}

// readContractTarget reads a contract target file into memory.
func readContractTarget(root, target string) (string, error) {
	path := filepath.Join(root, filepath.FromSlash(target))
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read contract target %s: %w", target, err)
	}
	return string(content), nil
}

// supportsCLIConfigAIFormat checks whether CLI config code mentions AI output support.
func supportsCLIConfigAIFormat(text string) bool {
	return strings.Contains(text, "FormatAI") || strings.Contains(strings.ToLower(text), "ai")
}

// violatesLifecycleDocsBoundary checks for forbidden file mutation calls.
func violatesLifecycleDocsBoundary(text string) bool {
	return strings.Contains(text, "os.WriteFile") || strings.Contains(text, "os.Rename")
}

// resolveAIAudienceContractTarget searches for a file that provides --ai/--user audience mode support.
func resolveAIAudienceContractTarget(root string, cfg config.PolicyConfig) (string, bool) {
	candidates := buildAIAudienceContractCandidates(cfg)

	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		trimmed, ok := normalizeNewContractCandidate(candidate, seen)
		if !ok {
			continue
		}
		if isAudienceContractCandidate(root, trimmed) {
			return trimmed, true
		}
	}
	return "", false
}

// buildAIAudienceContractCandidates returns configured and fallback audience-mode contract targets.
func buildAIAudienceContractCandidates(cfg config.PolicyConfig) []string {
	candidates := make([]string, 0, 6)
	candidates = appendConfiguredContractCandidate(candidates, cfg.Paths.ContractTargets, "audience_mode")
	candidates = appendConfiguredContractCandidate(candidates, cfg.Paths.ContractTargets, "root")
	candidates = appendConfiguredContractCandidate(candidates, cfg.Paths.ContractTargets, "app_run")
	return append(candidates, "cmd/root.go", "cmd/isr/main.go", "internal/app/run.go")
}

// appendConfiguredContractCandidate adds a non-empty configured contract target to the candidate list.
func appendConfiguredContractCandidate(candidates []string, targets map[string]string, key string) []string {
	if target, ok := targets[key]; ok && strings.TrimSpace(target) != "" {
		return append(candidates, target)
	}
	return candidates
}

// normalizeNewContractCandidate trims a candidate and filters empty or duplicate entries.
func normalizeNewContractCandidate(candidate string, seen map[string]struct{}) (string, bool) {
	trimmed := strings.TrimSpace(candidate)
	if trimmed == "" {
		return "", false
	}
	if _, ok := seen[trimmed]; ok {
		return "", false
	}
	seen[trimmed] = struct{}{}
	return trimmed, true
}

// isAudienceContractCandidate reports whether a file is a valid audience contract target.
func isAudienceContractCandidate(root, target string) bool {
	text, err := readContractTarget(root, target)
	if err != nil {
		return false
	}
	return hasAudienceModeSupport(text) || !isThinRootWrapper(text)
}

// hasAudienceModeSupport checks if text contains both --ai and --user flag patterns.
func hasAudienceModeSupport(text string) bool {
	return audienceModePatterns[0].MatchString(text) && audienceModePatterns[1].MatchString(text)
}

// isThinRootWrapper returns true if the text is a thin wrapper that delegates to another function.
func isThinRootWrapper(text string) bool {
	return strings.Contains(text, "RunCLI(") || strings.Contains(text, "app.RunCLI(")
}

// resolveContractTarget finds a contract target file path from config or fallback locations.
func resolveContractTarget(root string, cfg config.PolicyConfig, key string, fallbacks ...string) (string, bool) {
	if target, ok := cfg.Paths.ContractTargets[key]; ok && strings.TrimSpace(target) != "" {
		return target, true
	}
	for _, fallback := range fallbacks {
		if strings.TrimSpace(fallback) == "" {
			continue
		}
		if utils.PathExists(filepath.Join(root, filepath.FromSlash(fallback))) {
			return fallback, true
		}
	}
	if len(fallbacks) == 0 {
		return "", false
	}
	return fallbacks[0], true
}
