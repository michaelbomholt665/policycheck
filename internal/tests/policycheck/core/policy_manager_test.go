// internal/tests/policycheck/core/policy_manager_test.go
package core_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/core"
	"policycheck/internal/policycheck/types"
)

func TestRunPolicyChecks(t *testing.T) {
	// Mock a small registry
	originalRegistry := core.PolicyRegistry
	defer func() { core.PolicyRegistry = originalRegistry }()

	core.PolicyRegistry = map[string][]core.PolicyCheckFunc{
		"mock_group": {
			func(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation {
				return []types.Violation{
					{RuleID: "mock.rule1", Message: "Violation 1"},
				}
			},
			func(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation {
				return []types.Violation{
					{RuleID: "mock.rule2", Message: "Violation 2"},
				}
			},
		},
	}

	cfg := config.PolicyConfig{}
	violations := core.RunPolicyChecks(context.Background(), ".", cfg)

	assert.Len(t, violations, 2)
	assert.ElementsMatch(t, []string{"mock.rule1", "mock.rule2"}, []string{violations[0].RuleID, violations[1].RuleID})
}
