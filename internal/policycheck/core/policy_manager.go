// internal/policycheck/core/policy_manager.go
// Package core implements the central policy execution logic.
// It orchestrates the running of registered checks against the codebase.
package core

import (
	"context"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/types"
)

// RunPolicyChecks is the top-level orchestrator that iterates through the PolicyRegistry,
// executes every registered PolicyCheckFunc, and aggregates all types.Violation results.
func RunPolicyChecks(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation {
	violations := []types.Violation{}

	// Iterate through policy groups in the registry
	for _, groupChecks := range PolicyRegistry {
		for _, checkFunc := range groupChecks {
			results := checkFunc(ctx, root, cfg)
			if len(results) > 0 {
				violations = append(violations, results...)
			}
		}
	}

	return violations
}
