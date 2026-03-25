// internal/policycheck/core/checks.go
// Orchestrates all policy checks and aggregates results into a PolicyCheckResults.

package core

const ScopeProjectRepo = true


import (
	"sort"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/types"
)

// RunPolicyChecks executes all policy checks against the given root directory and config.
func RunPolicyChecks(root string, cfg config.PolicyConfig, scannerBytes types.ScannerBytes) types.PolicyCheckResults {
	results := types.PolicyCheckResults{}
	results.Violations = append(results.Violations, CheckGoVersion(root, cfg.GoVersion)...)
	results.Violations = append(results.Violations, CheckPythonVersion(root, cfg.PythonVersion)...)
	results.Violations = append(results.Violations, CheckTypescriptVersion(root, cfg.TypescriptVersion)...)
	results.Violations = append(results.Violations, CheckAIOutputContractPolicies(root, cfg)...)
	results.Violations = append(results.Violations, CheckIsrScopeBoundary(root, cfg)...)
	results.Violations = append(results.Violations, CheckPackageTopologyPolicies(root)...)
	results.Violations = append(results.Violations, CheckArchitecturePolicies(root, cfg)...)
	results.Violations = append(results.Violations, CheckSecretLoggingPolicies(root, cfg)...)
	results.Violations = append(results.Violations, CheckCLIOutputFormatterPolicies(root, cfg)...)
	results.Violations = append(results.Violations, CheckSymbolCommentPolicies(root)...)
	results.Violations = append(results.Violations, CheckTestFileLocation(root, cfg)...)

	fileWarnings, fileViolations := CheckCoreFileLOCPolicies(root, cfg)
	results.Warnings = append(results.Warnings, fileWarnings...)
	results.Violations = append(results.Violations, fileViolations...)

	scanErrs, warnings, violations := appendFunctionPolicyResults(root, cfg, scannerBytes, results.Warnings, results.Violations)
	results.ScannerErrors = scanErrs
	results.Warnings = warnings
	results.Violations = violations
	results.Warnings = append(results.Warnings, CheckHardcodedRuntimeKnobWarnings(root, cfg)...)

	sortViolations(results.ScannerErrors)
	sortViolations(results.Warnings)
	sortViolations(results.Violations)
	return results
}

// appendFunctionPolicyResults runs function quality policy checks and appends results to warnings and violations.
func appendFunctionPolicyResults(
	root string,
	cfg config.PolicyConfig,
	scannerBytes types.ScannerBytes,
	warnings, violations []types.Violation,
) ([]types.Violation, []types.Violation, []types.Violation) {
	scannerErrors, functionWarnings, functionViolations := CheckCoreFunctionLOCPolicies(root, cfg, scannerBytes)
	warnings = appendFunctionWarnings(cfg, warnings, functionWarnings)
	violations = append(violations, functionViolations...)
	return scannerErrors, warnings, violations
}

// sortViolations sorts violation items by path and message for consistent output.
func sortViolations(items []types.Violation) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Path == items[j].Path {
			return items[i].Message < items[j].Message
		}
		return items[i].Path < items[j].Path
	})
}
