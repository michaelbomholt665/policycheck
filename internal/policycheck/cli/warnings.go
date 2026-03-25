// internal/policycheck/cli/warnings.go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/types"
	"policycheck/internal/router/capabilities"
)

// PrintViolations prints the policy check violations to stdout using the standardized format.
// Template: [LEVEL] FILE:SYMBOL:LINE: MESSAGE [RULE_ID]
func PrintViolations(styler capabilities.CLIOutputStyler, violations []types.Violation) {
	seen := make(map[string]bool)
	for _, v := range violations {
		// Deduplicate: same file, function, line, and rule/message
		key := fmt.Sprintf("%s:%d:%s:%s:%s", v.File, v.Line, v.Function, v.RuleID, v.Message)
		if seen[key] {
			continue
		}
		seen[key] = true

		// Determine style kind
		kind := capabilities.TextKindWarning
		if v.Severity == "error" {
			kind = capabilities.TextKindError
		}

		// Get the styled gutter (prefix) from the styler
		prefix := "[WARN] "
		if v.Severity == "error" {
			prefix = "[ERROR] "
		}
		if styler != nil {
			if s, err := styler.StyleText(kind, ""); err == nil {
				prefix = s
			}
		}

		// Prepare context parts (FILE:SYMBOL:LINE)
		path := filepath.ToSlash(v.File)
		symbol := v.Function
		line := v.Line

		// Build context string based on what's available
		var context string
		if path != "" {
			if symbol != "" {
				context = fmt.Sprintf("%s:%s:%d:", path, symbol, line)
			} else if line > 0 {
				context = fmt.Sprintf("%s:%d:", path, line)
			} else {
				context = fmt.Sprintf("%s:", path)
			}
		}

		// Apply muted style to context if styler is available
		if styler != nil && context != "" {
			if s, err := styler.StyleText(capabilities.TextKindMuted, context); err == nil {
				context = s
			}
		}

		// Final output: [LEVEL] CONTEXT MESSAGE [RULE_ID]
		// The message itself is styled with the level's color (HiWhite in prettystyle)
		messageWithRule := fmt.Sprintf("%s [%s]", v.Message, v.RuleID)
		if styler != nil {
			// We pass only the message part to StyleText to get the color, 
			// but we skip the gutter since we already have it in 'prefix'
			// This avoids double-prefixing.
			// Actually, prettystyle's StyleText ALWAYS adds the gutter.
			// So we should just use StyleText for the whole message part.
			
			styledMsg, err := styler.StyleText(kind, messageWithRule)
			if err == nil {
				// styledMsg already contains the gutter and the HiWhite message
				if context != "" {
					// We need to insert the context AFTER the gutter.
					// prettystyle gutters are 9 characters (including spaces/codes).
					// But they are escape-coded, so we can't easily split by index.
					// However, we know it returns gutter + styledInput.
					// So if we pass empty string, we get JUST the gutter.
					gutter, _ := styler.StyleText(kind, "")
					fmt.Fprintf(os.Stdout, "%s%s %s\n", gutter, context, messageWithRule)
				} else {
					fmt.Fprintln(os.Stdout, styledMsg)
				}
				continue
			}
		}

		// Fallback for no styler or error
		if context != "" {
			fmt.Fprintf(os.Stdout, "%s%s %s\n", prefix, context, messageWithRule)
		} else {
			fmt.Fprintf(os.Stdout, "%s%s\n", prefix, messageWithRule)
		}
	}
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

	// If there are fewer than the minimum required for summarization,
	// restore them back as regular warnings (but use the standard RuleID for output)
	if len(mildCTX) < cfg.Output.MildCTXSummaryMinFunctions {
		for i := range mildCTX {
			mildCTX[i].RuleID = "function-quality"
		}
		return append(others, mildCTX...)
	}

	// Bundle them into a single summary line
	count := len(mildCTX)
	summary := types.Violation{
		RuleID:   "function-quality",
		Message:  fmt.Sprintf("%d functions have low CTX violations (CTX %d-%d)", count, cfg.FunctionQuality.MildCTXMin, cfg.FunctionQuality.ElevatedCTXMin-1),
		Severity: "warn",
	}

	return append(others, summary)
}
