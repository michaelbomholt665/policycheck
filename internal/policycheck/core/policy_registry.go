// internal/policycheck/core/policy_registry.go
// Package core implements the core policy engine.
// It maintains the registry of all active policy checks and their categories.
package core

import (
	"context"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/core/contracts"
	"policycheck/internal/policycheck/core/custom"
	"policycheck/internal/policycheck/core/hygiene"
	"policycheck/internal/policycheck/core/quality"
	"policycheck/internal/policycheck/core/security"
	"policycheck/internal/policycheck/core/structure"
	"policycheck/internal/policycheck/types"
)

// PolicyCheckFunc is a function that executes a specific policy check.
type PolicyCheckFunc func(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation

// PolicyRegistry maintains a list of all active policy checks.
var PolicyRegistry = map[string][]PolicyCheckFunc{
	"contracts": {
		contracts.CheckGoVersion,
		contracts.CheckCLIFormatter,
		contracts.CheckAICompatibility,
		contracts.CheckScopeGuard,
		contracts.CheckHardcodedRuntimeKnobs,
	},
	"quality": {
		quality.CheckFileSizePolicies,
		quality.CheckFunctionQualityPolicies,
	},
	"security": {
		security.CheckSecretLoggingPolicies,
	},
	"hygiene": {
		hygiene.CheckSymbolNames,
		hygiene.CheckDocStyle,
		hygiene.CheckDocumentation,
	},
	"structure": {
		structure.CheckTestLocation,
		structure.CheckPackageRules,
		structure.CheckArchitecture,
		structure.CheckRouterImports,
	},
	"custom": {
		custom.CheckCustomRules,
	},
}
