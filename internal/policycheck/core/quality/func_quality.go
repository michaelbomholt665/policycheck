// internal/policycheck/core/quality/func_quality.go
// Package quality implements code quality policy checks.
// It verifies function-level metrics like complexity and parameter counts.
package quality

import (
	"context"
	"fmt"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/host"
	"policycheck/internal/policycheck/types"
	"policycheck/internal/policycheck/utils"
)

// CheckFunctionQualityPolicies evaluates function-level quality facts against thresholds.
func CheckFunctionQualityPolicies(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation {
	scanner, err := host.ResolveScannerProvider()
	if err != nil {
		return []types.Violation{{
			RuleID:   "function-quality",
			Message:  fmt.Sprintf("resolve scanner provider: %v", err),
			Severity: "error",
		}}
	}

	facts, err := scanner.RunScanners(ctx, root)
	if err != nil {
		return []types.Violation{{
			RuleID:   "function-quality",
			Message:  fmt.Sprintf("run scanners: %v", err),
			Severity: "error",
		}}
	}

	facts = filterFunctionQualityFactsByRoots(facts, cfg.Paths.FunctionQualityRoots)

	return EvaluateFunctionQualityFacts(facts, cfg)
}

// filterFunctionQualityFactsByRoots limits evaluation to files within specified roots.
func filterFunctionQualityFactsByRoots(facts []types.PolicyFact, roots []string) []types.PolicyFact {
	if len(roots) == 0 {
		return facts
	}

	filtered := make([]types.PolicyFact, 0, len(facts))
	for _, fact := range facts {
		if utils.HasPrefix(fact.FilePath, roots) {
			filtered = append(filtered, fact)
		}
	}

	return filtered
}

// EvaluateFunctionQualityFacts evaluates extracted facts against configured thresholds.
func EvaluateFunctionQualityFacts(facts []types.PolicyFact, cfg config.PolicyConfig) []types.Violation {
	var viols []types.Violation

	enabled := make(map[string]bool)
	for _, lang := range cfg.FunctionQuality.EnabledLanguages {
		enabled[lang] = true
	}

	for _, fact := range facts {
		if !enabled[fact.Language] || (fact.SymbolKind != "function" && fact.SymbolKind != "method") {
			continue
		}

		factViols, factMildWarnings := evaluateSingleFact(fact, cfg.FunctionQuality)
		viols = append(viols, factViols...)
		viols = append(viols, factMildWarnings...)
	}

	return viols
}

// evaluateSingleFact checks a single function's metrics against all quality thresholds.
func evaluateSingleFact(fact types.PolicyFact, cfg config.PolicyFunctionQualityConfig) ([]types.Violation, []types.Violation) {
	var viols []types.Violation
	var mildWarnings []types.Violation
	loc := fact.EndLine - fact.LineNumber + 1

	warnLOC, maxLOC := resolveLanguageLOCLimits(fact.Language, cfg)

	if fact.Complexity >= cfg.ErrorCTXAndLOCCTX && loc >= cfg.ErrorCTXAndLOCLOC {
		return []types.Violation{newFuncViolation(fact, "error", fmt.Sprintf("function %s is both complex and long (CTX:%d, LOC:%d); must be refactored", fact.SymbolName, fact.Complexity, loc))}, nil
	}

	if fact.Complexity >= cfg.ErrorCTXMin {
		return []types.Violation{newFuncViolation(fact, "error", fmt.Sprintf("function %s is excessively complex (CTX:%d); hard limit is %d", fact.SymbolName, fact.Complexity, cfg.ErrorCTXMin))}, nil
	}

	if loc >= maxLOC {
		return []types.Violation{newFuncViolation(fact, "error", fmt.Sprintf("function %s is excessively long (LOC:%d); hard limit is %d", fact.SymbolName, loc, maxLOC))}, nil
	}

	viols, mildWarnings = appendComplexityViolations(viols, mildWarnings, fact, cfg)

	if loc >= warnLOC {
		viols = append(viols, newFuncViolation(fact, "warn", fmt.Sprintf("function %s is approaching size limit (LOC:%d); warn threshold is %d", fact.SymbolName, loc, warnLOC)))
	}

	viols = appendParameterViolations(viols, fact, cfg)

	if fact.RepeatedNilGuards >= cfg.NilGuardRepeatWarnCount {
		viols = append(viols, newFuncViolation(fact, "warn", fmt.Sprintf("function %s repeats plain nil-guard checks %d times; CTX may be inflated by distant repeated guards", fact.SymbolName, fact.RepeatedNilGuards)))
	}

	return viols, mildWarnings
}

// resolveLanguageLOCLimits determined the applicable LOC limits for a given language.
func resolveLanguageLOCLimits(language string, cfg config.PolicyFunctionQualityConfig) (int, int) {
	switch language {
	case "python":
		return cfg.PythonWarnLOC, cfg.PythonMaxLOC
	case "typescript":
		return cfg.TypeScriptWarnLOC, cfg.TypeScriptMaxLOC
	case "go":
		return cfg.GoWarnLOC, cfg.GoMaxLOC
	default:
		return cfg.WarnLOC, cfg.MaxLOC
	}
}

// appendComplexityViolations checks complexity thresholds and adds weighted-warnings or errors.
func appendComplexityViolations(
	viols []types.Violation,
	mildWarnings []types.Violation,
	fact types.PolicyFact,
	cfg config.PolicyFunctionQualityConfig,
) ([]types.Violation, []types.Violation) {
	if fact.Complexity >= cfg.ImmediateRefactorCTXMin {
		viols = append(viols, newFuncViolation(fact, "warn", fmt.Sprintf("function %s requires immediate refactoring (CTX:%d); exceeds refactor threshold %d", fact.SymbolName, fact.Complexity, cfg.ImmediateRefactorCTXMin)))
		return viols, mildWarnings
	}

	if fact.Complexity >= cfg.ElevatedCTXMin {
		viols = append(viols, newFuncViolation(fact, "warn", fmt.Sprintf("function %s has elevated complexity (CTX:%d); exceeds elevated threshold %d", fact.SymbolName, fact.Complexity, cfg.ElevatedCTXMin)))
		return viols, mildWarnings
	}

	if fact.Complexity >= cfg.MildCTXMin {
		v := newFuncViolation(fact, "warn", fmt.Sprintf("function %s has mild complexity (CTX:%d); exceeds mild threshold %d", fact.SymbolName, fact.Complexity, cfg.MildCTXMin))
		v.RuleID = "function-quality.mild-ctx"
		mildWarnings = append(mildWarnings, v)
	}

	return viols, mildWarnings
}

// appendParameterViolations checks the function's parameter count against limits.
func appendParameterViolations(
	viols []types.Violation,
	fact types.PolicyFact,
	cfg config.PolicyFunctionQualityConfig,
) []types.Violation {
	if fact.ParamCount >= cfg.MaxParameterCount {
		return append(viols, newFuncViolation(fact, "error", fmt.Sprintf("function %s has excessive parameters (%d); hard limit is %d", fact.SymbolName, fact.ParamCount, cfg.MaxParameterCount)))
	}

	if fact.ParamCount >= cfg.WarnParameterCount {
		return append(viols, newFuncViolation(fact, "warn", fmt.Sprintf("function %s has many parameters (%d); warn threshold is %d", fact.SymbolName, fact.ParamCount, cfg.WarnParameterCount)))
	}

	return viols
}

// newFuncViolation creates a standardized quality violation for a function.
func newFuncViolation(fact types.PolicyFact, severity, msg string) types.Violation {
	return types.Violation{
		RuleID:   "function-quality",
		File:     fact.FilePath,
		Function: fact.SymbolName,
		Line:     fact.LineNumber,
		Message:  msg,
		Severity: severity,
	}
}
