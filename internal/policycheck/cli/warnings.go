// internal/policycheck/cli/warnings.go
// Package cli/warnings implements the logic for formatting and summarizing policy violations.
// It includes support for contextual prefixing, deduplication, and mild-CTX compression.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/types"
	"policycheck/internal/router/capabilities"

	"github.com/jedib0t/go-pretty/v6/text"
)

const userWrapWidth = 100

// PrintViolations prints the policy check violations to stdout using the standardized format.
//
// Template: [LEVEL] FILE:SYMBOL:LINE: MESSAGE [RULE_ID]
func PrintViolations(chrome capabilities.CLIChromeStyler, outputMode string, violations []types.Violation) error {
	if outputMode == outputModeUser {
		return printUserViolations(chrome, violations)
	}

	return printAIViolations(chrome, violations)
}

// printAIViolations renders the compact line-oriented violation format.
func printAIViolations(chrome capabilities.CLIChromeStyler, violations []types.Violation) error {
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

// printUserViolations renders grouped file panels for interactive terminal use.
func printUserViolations(chrome capabilities.CLIChromeStyler, violations []types.Violation) error {
	grouped := groupViolationsByFile(violations)
	if err := printUserSummary(chrome, grouped); err != nil {
		return err
	}

	for index, group := range grouped {
		if index > 0 {
			if _, err := fmt.Fprintln(os.Stdout); err != nil {
				return fmt.Errorf("write file group separator: %w", err)
			}
		}

		rendered, err := renderUserViolationGroup(chrome, group)
		if err != nil {
			return fmt.Errorf("render user violation group for %s: %w", group.file, err)
		}

		if _, err := fmt.Fprintln(os.Stdout, rendered); err != nil {
			return fmt.Errorf("write user violation group for %s: %w", group.file, err)
		}
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

type violationGroup struct {
	file       string
	violations []types.Violation
}

// groupViolationsByFile deduplicates and clusters violations by file path.
func groupViolationsByFile(violations []types.Violation) []violationGroup {
	seen := make(map[string]bool)
	grouped := make(map[string][]types.Violation)
	order := make([]string, 0)

	for _, violation := range violations {
		key := fmt.Sprintf(
			"%s:%d:%s:%s:%s",
			violation.File,
			violation.Line,
			violation.Function,
			violation.RuleID,
			violation.Message,
		)
		if seen[key] {
			continue
		}
		seen[key] = true

		fileKey := filepath.ToSlash(violation.File)
		if strings.TrimSpace(fileKey) == "" {
			fileKey = "(repo)"
		}
		if _, ok := grouped[fileKey]; !ok {
			order = append(order, fileKey)
		}
		grouped[fileKey] = append(grouped[fileKey], violation)
	}

	sort.Slice(order, func(i, j int) bool {
		if order[i] == "(repo)" {
			return false
		}
		if order[j] == "(repo)" {
			return true
		}
		return order[i] < order[j]
	})

	result := make([]violationGroup, 0, len(order))
	for _, fileKey := range order {
		result = append(result, violationGroup{
			file:       fileKey,
			violations: grouped[fileKey],
		})
	}

	return result
}

// printUserSummary renders the report-level totals for user mode.
func printUserSummary(chrome capabilities.CLIChromeStyler, groups []violationGroup) error {
	errorCount := 0
	warnCount := 0
	for _, group := range groups {
		for _, violation := range group.violations {
			if violation.Severity == "error" {
				errorCount++
				continue
			}
			warnCount++
		}
	}

	title := fmt.Sprintf(
		"Policycheck Report  %d error(s)  %d warning(s)  %d file(s)",
		errorCount,
		warnCount,
		len(groups),
	)
	rendered, err := styleChromeText(chrome, capabilities.TextKindHeader, title)
	if err != nil {
		rendered = title
	}

	if _, err := fmt.Fprintln(os.Stdout, rendered); err != nil {
		return fmt.Errorf("write user summary: %w", err)
	}

	if _, err := fmt.Fprintln(os.Stdout); err != nil {
		return fmt.Errorf("write user summary spacer: %w", err)
	}

	return nil
}

// renderUserViolationGroup renders one file-scoped group without width-sensitive borders.
func renderUserViolationGroup(chrome capabilities.CLIChromeStyler, group violationGroup) (string, error) {
	header, err := styleChromeText(chrome, capabilities.TextKindHeader, group.file)
	if err != nil {
		header = group.file
	}

	lines := make([]string, 0, 1+len(group.violations)*3)
	lines = append(lines, header)
	for _, violation := range group.violations {
		titleKind := capabilities.TextKindWarning
		if violation.Severity == "error" {
			titleKind = capabilities.TextKindError
		}

		titleText := "  - " + userViolationHeadline(violation)
		styledTitle, err := styleChromeText(chrome, titleKind, titleText)
		if err != nil {
			styledTitle = titleText
		}

		detailText := userViolationDetail(violation)
		styledDetail, err := styleChromeText(chrome, capabilities.TextKindMuted, detailText)
		if err != nil {
			styledDetail = detailText
		}

		lines = append(lines, wrapUserStyledLine(styledTitle, "    "))
		if styledDetail != "" {
			lines = append(lines, wrapUserStyledLine("    "+styledDetail, "      "))
		}
	}

	return strings.Join(lines, "\n"), nil
}

// userViolationHeadline returns the primary summary line for one violation.
func userViolationHeadline(v types.Violation) string {
	if v.Line > 0 {
		return fmt.Sprintf("Line %d: %s", v.Line, v.Message)
	}
	return v.Message
}

// userViolationDetail returns muted supplemental context for one violation.
func userViolationDetail(v types.Violation) string {
	parts := make([]string, 0, 2)
	if v.Function != "" {
		parts = append(parts, fmt.Sprintf("Symbol: %s", v.Function))
	}

	rule := formatRuleLabel(v.RuleID)
	if rule != "" {
		parts = append(parts, fmt.Sprintf("[%s]", rule))
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "  ")
}

// wrapUserStyledLine wraps one user-mode output line with a hanging indent.
func wrapUserStyledLine(line string, continuationIndent string) string {
	if text.RuneWidthWithoutEscSequences(line) <= userWrapWidth {
		return line
	}

	wrapped := text.WrapSoft(line, userWrapWidth)
	if !strings.Contains(wrapped, "\n") {
		return wrapped
	}

	parts := strings.Split(wrapped, "\n")
	for index := 1; index < len(parts); index++ {
		if strings.TrimSpace(parts[index]) == "" {
			continue
		}
		parts[index] = continuationIndent + strings.TrimLeft(parts[index], " ")
	}

	return strings.Join(parts, "\n")
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
