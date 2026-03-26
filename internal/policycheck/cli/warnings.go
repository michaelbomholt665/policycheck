// internal/policycheck/cli/warnings.go
// Package cli/warnings implements the logic for formatting and summarizing policy violations.
// It includes support for contextual prefixing, deduplication, and mild-CTX compression.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/types"
	"policycheck/internal/router/capabilities"
)

// PrintViolations prints the policy check violations to stdout using the standardized format.
//
// Template: [LEVEL] FILE:SYMBOL:LINE: MESSAGE [RULE_ID]
func PrintViolations(chrome capabilities.CLIChromeStyler, violations []types.Violation) error {
	seen := make(map[string]bool)
	previousGroupKey := ""
	for _, v := range violations {
		// Deduplicate: same file, function, line, and rule/message
		key := fmt.Sprintf("%s:%d:%s:%s:%s", v.File, v.Line, v.Function, v.RuleID, v.Message)
		if seen[key] {
			continue
		}
		seen[key] = true

		currentGroupKey := fmt.Sprintf("%s:%s", v.Severity, v.RuleID)
		if previousGroupKey != "" && previousGroupKey != currentGroupKey {
			if _, err := fmt.Fprintln(os.Stdout); err != nil {
				return fmt.Errorf("write violation group separator: %w", err)
			}
		}

		if err := printSingleViolation(chrome, v); err != nil {
			return fmt.Errorf("print violation %s: %w", v.RuleID, err)
		}

		previousGroupKey = currentGroupKey
	}

	return nil
}

// printSingleViolation renders a single violation with its severity prefix and location context.
func printSingleViolation(chrome capabilities.CLIChromeStyler, v types.Violation) error {
	prefix, context := buildViolationPrefixAndContext(chrome, v)
	messageWithRule := fmt.Sprintf("%s [%s]", v.Message, v.RuleID)

	if context != "" {
		if _, err := fmt.Fprintf(os.Stdout, "%s%s %s\n", prefix, context, messageWithRule); err != nil {
			return fmt.Errorf("write contextual violation line: %w", err)
		}

		return nil
	}

	if _, err := fmt.Fprintf(os.Stdout, "%s%s\n", prefix, messageWithRule); err != nil {
		return fmt.Errorf("write violation line: %w", err)
	}

	return nil
}

// buildViolationPrefixAndContext constructs the stylized severity prefix and location string for a violation.
func buildViolationPrefixAndContext(chrome capabilities.CLIChromeStyler, v types.Violation) (string, string) {
	kind := capabilities.TextKindWarning
	if v.Severity == "error" {
		kind = capabilities.TextKindError
	}

	prefix := "[WARN] "
	if v.Severity == "error" {
		prefix = "[ERROR] "
	}
	if chrome != nil {
		if s, err := chrome.StyleText(kind, ""); err == nil {
			prefix = s
		}
	}

	context := buildViolationContextString(v)
	if chrome != nil && context != "" {
		if s, err := chrome.StyleText(capabilities.TextKindMuted, context); err == nil {
			context = s
		}
	}
	return prefix, context
}

// buildViolationContextString generates the file:line or file:func:line location string.
func buildViolationContextString(v types.Violation) string {
	path := filepath.ToSlash(v.File)
	if path == "" {
		return ""
	}
	if v.Function != "" {
		return fmt.Sprintf("%s:%s:%d:", path, v.Function, v.Line)
	}
	if v.Line > 0 {
		return fmt.Sprintf("%s:%d:", path, v.Line)
	}
	return fmt.Sprintf("%s:", path)
}

// SummarizeWarnings bundles mild warnings consistently across all policy categories
// to minimize noise in large repositories.
func SummarizeWarnings(cfg config.PolicyConfig, violations []types.Violation) []types.Violation {
	if !cfg.Output.MildCTXCompressSummary {
		return violations
	}

	var mildCTX []types.Violation
	var others []types.Violation

	// Separate mild CTX warnings from everything else
	for _, v := range violations {
		if v.RuleID == "function-quality.mild-ctx" {
			mildCTX = append(mildCTX, v)
		} else {
			others = append(others, v)
		}
	}

	perFileSummaries, remainingMildCTX := summarizePerFileMildCTX(cfg, mildCTX)
	others = append(others, perFileSummaries...)

	// If there are fewer than the minimum required for global summarization,
	// restore them back as regular warnings (but use the standard RuleID for output)
	if len(remainingMildCTX) < cfg.Output.MildCTXSummaryMinFunctions {
		for i := range remainingMildCTX {
			remainingMildCTX[i].RuleID = "function-quality"
		}
		return append(others, remainingMildCTX...)
	}

	// Bundle remaining low CTX warnings into a single global summary line
	count := len(remainingMildCTX)
	summary := newGlobalMildCTXSummary(cfg, count)

	return append(others, summary)
}

// summarizePerFileMildCTX groups low-CTX warnings by file when they exceed the escalation threshold.
func summarizePerFileMildCTX(cfg config.PolicyConfig, mildCTX []types.Violation) ([]types.Violation, []types.Violation) {
	if cfg.Output.MildCTXPerFileSummaryMinCount <= 0 {
		return nil, mildCTX
	}

	grouped := make(map[string][]types.Violation)
	order := make([]string, 0)

	for _, violation := range mildCTX {
		key := filepath.ToSlash(violation.File)
		if _, ok := grouped[key]; !ok {
			order = append(order, key)
		}
		grouped[key] = append(grouped[key], violation)
	}

	sort.Strings(order)

	summaries := make([]types.Violation, 0, len(order))
	remaining := make([]types.Violation, 0, len(mildCTX))

	for _, key := range order {
		fileViols := grouped[key]
		if len(fileViols) < cfg.Output.MildCTXPerFileSummaryMinCount {
			remaining = append(remaining, fileViols...)
			continue
		}

		summaries = append(summaries, newPerFileMildCTXSummary(cfg, key, len(fileViols)))
	}

	return summaries, remaining
}

// newGlobalMildCTXSummary creates a single summary violation for all low-CTX warnings across the repo.
func newGlobalMildCTXSummary(cfg config.PolicyConfig, count int) types.Violation {
	return types.Violation{
		RuleID:   "function-quality",
		Message:  fmt.Sprintf("%d functions have low CTX violations (CTX %d-%d)", count, cfg.FunctionQuality.MildCTXMin, cfg.FunctionQuality.ElevatedCTXMin-1),
		Severity: "warn",
	}
}

// newPerFileMildCTXSummary creates a summary violation for all low-CTX warnings in a specific file.
func newPerFileMildCTXSummary(cfg config.PolicyConfig, file string, count int) types.Violation {
	message := fmt.Sprintf("%s has %d low CTX violations (CTX %d-%d)", file, count, cfg.FunctionQuality.MildCTXMin, cfg.FunctionQuality.ElevatedCTXMin-1)
	if cfg.Output.MildCTXPerFileEscalationCount > 0 && count >= cfg.Output.MildCTXPerFileEscalationCount {
		message = fmt.Sprintf("%s has %d low CTX violations (CTX %d-%d); hotspot threshold is %d", file, count, cfg.FunctionQuality.MildCTXMin, cfg.FunctionQuality.ElevatedCTXMin-1, cfg.Output.MildCTXPerFileEscalationCount)
	}

	return types.Violation{
		RuleID:   "function-quality",
		File:     file,
		Message:  message,
		Severity: "warn",
	}
}
