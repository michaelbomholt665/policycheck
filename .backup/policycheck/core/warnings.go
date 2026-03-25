// internal/policycheck/core/warnings.go
// Warning collection helpers for function quality warning summarization.

package core

const ScopeProjectRepo = true


import (
	"fmt"
	"sort"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/types"
)

// appendFunctionWarnings converts function quality warnings to violations and appends them to the list.
func appendFunctionWarnings(cfg config.PolicyConfig, warnings []types.Violation, functionWarnings []types.FunctionQualityWarning) []types.Violation {
	if !cfg.Output.MildCTXCompressSummary {
		for _, warning := range functionWarnings {
			warnings = append(warnings, types.Violation{Path: warning.Path, Message: warning.Message})
		}
		return warnings
	}

	immediateAndElevated, mildByFile := partitionFunctionWarnings(functionWarnings)
	for _, warning := range immediateAndElevated {
		warnings = append(warnings, types.Violation{Path: warning.Path, Message: warning.Message})
	}
	return appendSummarizedMildWarnings(cfg, warnings, mildByFile)
}

// partitionFunctionWarnings separates warnings into elevated and mild categories by file.
func partitionFunctionWarnings(functionWarnings []types.FunctionQualityWarning) ([]types.FunctionQualityWarning, map[string][]types.FunctionQualityWarning) {
	immediateAndElevated := []types.FunctionQualityWarning{}
	mildByFile := map[string][]types.FunctionQualityWarning{}

	for _, warning := range functionWarnings {
		if warning.Level >= types.FunctionQualityLevelElevated {
			immediateAndElevated = append(immediateAndElevated, warning)
		} else {
			mildByFile[warning.Path] = append(mildByFile[warning.Path], warning)
		}
	}
	return immediateAndElevated, mildByFile
}

// appendSummarizedMildWarnings adds mild complexity warnings with optional summarization.
func appendSummarizedMildWarnings(cfg config.PolicyConfig, warnings []types.Violation, mildByFile map[string][]types.FunctionQualityWarning) []types.Violation {
	filePaths := make([]string, 0, len(mildByFile))
	for filePath := range mildByFile {
		filePaths = append(filePaths, filePath)
	}
	sort.Strings(filePaths)

	for _, filePath := range filePaths {
		fileWarnings := mildByFile[filePath]
		if len(fileWarnings) >= cfg.Output.MildCTXPerFileEscalationCount {
			for _, warning := range fileWarnings {
				warnings = append(warnings, types.Violation{Path: warning.Path, Message: warning.Message})
			}
			continue
		}
		if len(fileWarnings) >= cfg.Output.MildCTXPerFileSummaryMinCount {
			warnings = append(warnings, types.Violation{
				Path:    filePath,
				Message: fmt.Sprintf("%d functions with mild CTX elevation; consider refactoring", len(fileWarnings)),
			})
			continue
		}
		for _, warning := range fileWarnings {
			warnings = append(warnings, types.Violation{Path: warning.Path, Message: warning.Message})
		}
	}
	return warnings
}
