package quality

import (
	"context"
	"fmt"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/host"
	"policycheck/internal/policycheck/types"
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

	return EvaluateFunctionQualityFacts(facts, cfg)
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

func evaluateSingleFact(fact types.PolicyFact, cfg config.PolicyFunctionQualityConfig) ([]types.Violation, []types.Violation) {
	var viols []types.Violation
	var mildWarnings []types.Violation
	loc := fact.EndLine - fact.LineNumber + 1

	var warnLOC, maxLOC int
	switch fact.Language {
	case "python":
		warnLOC, maxLOC = cfg.PythonWarnLOC, cfg.PythonMaxLOC
	case "typescript":
		warnLOC, maxLOC = cfg.TypeScriptWarnLOC, cfg.TypeScriptMaxLOC
	case "go":
		warnLOC, maxLOC = cfg.GoWarnLOC, cfg.GoMaxLOC
	default:
		warnLOC, maxLOC = cfg.WarnLOC, cfg.MaxLOC
	}

	if fact.Complexity >= cfg.ErrorCTXAndLOCCTX && loc >= cfg.ErrorCTXAndLOCLOC {
		return []types.Violation{newFuncViolation(fact, "error", fmt.Sprintf("function %s is both complex and long (CTX:%d, LOC:%d); must be refactored", fact.SymbolName, fact.Complexity, loc))}, nil
	}

	if fact.Complexity >= cfg.ErrorCTXMin {
		return []types.Violation{newFuncViolation(fact, "error", fmt.Sprintf("function %s is excessively complex (CTX:%d); hard limit is %d", fact.SymbolName, fact.Complexity, cfg.ErrorCTXMin))}, nil
	}

	if loc >= maxLOC {
		return []types.Violation{newFuncViolation(fact, "error", fmt.Sprintf("function %s is excessively long (LOC:%d); hard limit is %d", fact.SymbolName, loc, maxLOC))}, nil
	}

	if fact.Complexity >= cfg.ImmediateRefactorCTXMin {
		viols = append(viols, newFuncViolation(fact, "warn", fmt.Sprintf("function %s requires immediate refactoring (CTX:%d); exceeds refactor threshold %d", fact.SymbolName, fact.Complexity, cfg.ImmediateRefactorCTXMin)))
	} else if fact.Complexity >= cfg.ElevatedCTXMin {
		viols = append(viols, newFuncViolation(fact, "warn", fmt.Sprintf("function %s has elevated complexity (CTX:%d); exceeds elevated threshold %d", fact.SymbolName, fact.Complexity, cfg.ElevatedCTXMin)))
	} else if fact.Complexity >= cfg.MildCTXMin {
		v := newFuncViolation(fact, "warn", fmt.Sprintf("function %s has mild complexity (CTX:%d); exceeds mild threshold %d", fact.SymbolName, fact.Complexity, cfg.MildCTXMin))
		v.RuleID = "function-quality.mild-ctx"
		mildWarnings = append(mildWarnings, v)
	}

	if loc >= warnLOC {
		viols = append(viols, newFuncViolation(fact, "warn", fmt.Sprintf("function %s is approaching size limit (LOC:%d); warn threshold is %d", fact.SymbolName, loc, warnLOC)))
	}

	if fact.ParamCount >= cfg.MaxParameterCount {
		viols = append(viols, newFuncViolation(fact, "error", fmt.Sprintf("function %s has excessive parameters (%d); hard limit is %d", fact.SymbolName, fact.ParamCount, cfg.MaxParameterCount)))
	} else if fact.ParamCount >= cfg.WarnParameterCount {
		viols = append(viols, newFuncViolation(fact, "warn", fmt.Sprintf("function %s has many parameters (%d); warn threshold is %d", fact.SymbolName, fact.ParamCount, cfg.WarnParameterCount)))
	}

	if fact.RepeatedNilGuards >= cfg.NilGuardRepeatWarnCount {
		viols = append(viols, newFuncViolation(fact, "warn", fmt.Sprintf("function %s repeats plain nil-guard checks %d times; CTX may be inflated by distant repeated guards", fact.SymbolName, fact.RepeatedNilGuards)))
	}

	return viols, mildWarnings
}

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
