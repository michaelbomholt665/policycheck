// internal/policycheck/core/quality/file_size.go
// Package quality implements code quality policy checks.
// It monitors file size and enforces limits based on code complexity metrics.
//
// Package Concerns:
//   - File size monitoring with CTX-based penalties
//   - Function complexity and length (LOC) validation
//   - Nil-guard repetition detection
package quality

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/host"
	"policycheck/internal/policycheck/types"
)

// CheckFileSizePolicies evaluates file size limits across the repository.
func CheckFileSizePolicies(ctx context.Context, root string, cfg config.PolicyConfig) []types.Violation {
	var viols []types.Violation

	walk, err := host.ResolveWalkProvider()
	if err != nil {
		return appendErrorViol(viols, "file-size", fmt.Sprintf("resolve walk provider: %v", err))
	}
	walker, ok := walk.(directoryWalker)
	if !ok {
		return appendErrorViol(viols, "file-size", "walk provider does not satisfy directoryWalker")
	}

	scanner, err := host.ResolveScannerProvider()
	if err != nil {
		return appendErrorViol(viols, "file-size", fmt.Sprintf("resolve scanner provider: %v", err))
	}

	// For file size penalty, we need complexity facts for each file.
	// In a real implementation, we might cache these from a single scanner run.
	facts, err := scanner.RunScanners(ctx, root)
	if err != nil {
		return appendErrorViol(viols, "file-size", fmt.Sprintf("run scanners: %v", err))
	}

	warnCtxFuncs, hardCtxFuncs := calculateComplexityPenalties(facts, cfg)

	fileViols, err := walkAndEvaluateFileSize(root, cfg, walker, warnCtxFuncs, hardCtxFuncs)
	if err != nil {
		viols = appendErrorViol(viols, "file-size", fmt.Sprintf("walk directory: %v", err))
	}
	viols = append(viols, fileViols...)

	return viols
}

// calculateComplexityPenalties aggregates function complexity into per-file penalty counts.
func calculateComplexityPenalties(facts []types.PolicyFact, cfg config.PolicyConfig) (warnCtxFuncs, hardCtxFuncs map[string]int) {
	warnCtxFuncs = make(map[string]int)
	hardCtxFuncs = make(map[string]int)
	for _, fact := range facts {
		if fact.Complexity >= cfg.FunctionQuality.MildCTXMin {
			warnCtxFuncs[fact.FilePath]++
		}
		if fact.Complexity >= cfg.FileSize.MaxPenaltyCTXThreshold {
			hardCtxFuncs[fact.FilePath]++
		}
	}
	return warnCtxFuncs, hardCtxFuncs
}

type directoryWalker interface {
	WalkDirectoryTree(root string, fn fs.WalkDirFunc) error
}

// walkAndEvaluateFileSize traverses the directory tree and evaluates each file's size.
func walkAndEvaluateFileSize(root string, cfg config.PolicyConfig, walk directoryWalker, warnCtxFuncs, hardCtxFuncs map[string]int) ([]types.Violation, error) {
	var viols []types.Violation
	err := walk.WalkDirectoryTree(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		rel, _ := host.RelOrAbs(root, path)

		if !host.HasPrefix(rel, cfg.Paths.FileLOCRoots) || host.HasPrefix(rel, cfg.Paths.LOCIgnorePrefixes) {
			return nil
		}

		content, readErr := host.ReadFile(path)
		if readErr != nil {
			return nil // Skip files we cannot read
		}
		lineCount := len(strings.Split(string(content), "\n"))

		warnLOC, maxLOC := ComputeFileSizeThresholds(cfg.FileSize, warnCtxFuncs[rel], hardCtxFuncs[rel])
		viols = append(viols, EvaluateFileSize(rel, lineCount, warnLOC, maxLOC)...)

		return nil
	})
	return viols, err
}

// appendErrorViol is a helper to add an error-level violation to a results slice.
func appendErrorViol(viols []types.Violation, ruleID, msg string) []types.Violation {
	return append(viols, types.Violation{
		RuleID:   ruleID,
		Message:  msg,
		Severity: "error",
	})
}

// ComputeFileSizeThresholds calculates effective LOC limits based on CTX-based penalties.
func ComputeFileSizeThresholds(cfg config.PolicyFileSizeConfig, warnCtxFuncCount, hardCtxFuncCount int) (warnLOC, maxLOC int) {
	effectiveWarn := cfg.WarnLOC - (warnCtxFuncCount * cfg.WarnPenaltyPerCTXFunction)
	if effectiveWarn < cfg.MinWarnLOC {
		effectiveWarn = cfg.MinWarnLOC
	}

	effectiveMax := cfg.MaxLOC - (hardCtxFuncCount * cfg.MaxPenaltyPerCTXFunction)
	if effectiveMax < cfg.MinMaxLOC {
		effectiveMax = cfg.MinMaxLOC
	}

	// Ensure gap
	if effectiveMax < effectiveWarn+cfg.MinWarnToMaxGap {
		effectiveMax = effectiveWarn + cfg.MinWarnToMaxGap
	}

	return effectiveWarn, effectiveMax
}

// EvaluateFileSize checks a single file's line count against pre-computed thresholds.
func EvaluateFileSize(rel string, lineCount, warnLOC, maxLOC int) []types.Violation {
	if lineCount > maxLOC {
		return []types.Violation{{
			RuleID:   "file-size",
			File:     rel,
			Message:  fmt.Sprintf("file %s is too large: %d lines (max allowed: %d lines with complexity penalty)", rel, lineCount, maxLOC),
			Severity: "error",
		}}
	}
	if lineCount > warnLOC {
		return []types.Violation{{
			RuleID:   "file-size",
			File:     rel,
			Message:  fmt.Sprintf("file %s is approaching size limit: %d lines (warn threshold: %d lines with complexity penalty)", rel, lineCount, warnLOC),
			Severity: "warn",
		}}
	}
	return nil
}
