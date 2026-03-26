// internal/cliwrapper/severity.go
// Evaluates wrapper security severities and blocking decisions for advisories.
// Keeps severity threshold logic pure so callers can test decisions deterministically.
package cliwrapper

import "fmt"

// SecurityDecision is the outcome of a security gate evaluation.
type SecurityDecision int

const (
	// DecisionAllow means the package(s) may proceed.
	DecisionAllow SecurityDecision = iota
	// DecisionBlock means at least one advisory exceeds the block threshold.
	DecisionBlock
	// DecisionScannerFailure means the scan could not complete; treated as block.
	DecisionScannerFailure
)

// Advisory is a single vulnerability record returned from the security backend.
type Advisory struct {
	// ID is the advisory identifier (CVE, GHSA, OSV ID, etc.).
	ID string
	// Summary is a short human-readable description.
	Summary string
	// Severity is the string severity label (e.g. "critical", "high").
	Severity string
}

// SecurityResult is the structured outcome of a gate evaluation.
type SecurityResult struct {
	// Decision is the gate action derived from the advisory list and threshold.
	Decision SecurityDecision
	// Advisories is the full list of advisories evaluated.
	Advisories []Advisory
	// BlockReason is a human-readable explanation when Decision != DecisionAllow.
	BlockReason string
	// BlockingSeverity is the advisory severity that triggered the block.
	BlockingSeverity Severity
}

// EvaluateSeverity applies threshold to advisories and returns a SecurityResult.
//
// EvaluateSeverity is a pure function: no I/O, deterministic, safe for tests.
// At least one advisory at or above threshold produces DecisionBlock.
func EvaluateSeverity(threshold Severity, advisories []Advisory) SecurityResult {
	for _, adv := range advisories {
		advSev, err := ParseSeverity(adv.Severity)
		if err != nil {
			// Unrecognised severity strings are treated conservatively as critical.
			advSev = SeverityCritical
		}

		if SeverityAtLeast(advSev, threshold) {
			return SecurityResult{
				Decision:         DecisionBlock,
				Advisories:       advisories,
				BlockReason:      fmt.Sprintf("advisory %s (%s) meets or exceeds block threshold", adv.ID, adv.Severity),
				BlockingSeverity: advSev,
			}
		}
	}

	return SecurityResult{
		Decision:   DecisionAllow,
		Advisories: advisories,
	}
}

// EvaluateSecurityPolicy applies an explicit block_on config to advisories.
func EvaluateSecurityPolicy(cfg WrapperSecurityConfig, advisories []Advisory) SecurityResult {
	blockLevels := cfg.BlockOn
	if len(blockLevels) == 0 {
		blockLevels = DefaultWrapperConfig().Security.BlockOn
	}

	for _, advisory := range advisories {
		advisorySeverity, err := ParseSeverity(advisory.Severity)
		if err != nil {
			advisorySeverity = SeverityCritical
		}

		for _, blockedLabel := range blockLevels {
			blockedSeverity, err := ParseSeverity(blockedLabel)
			if err != nil {
				blockedSeverity = SeverityCritical
			}

			if advisorySeverity == blockedSeverity {
				return SecurityResult{
					Decision:         DecisionBlock,
					Advisories:       advisories,
					BlockReason:      fmt.Sprintf("advisory %s (%s) is blocked by security.block_on", advisory.ID, advisory.Severity),
					BlockingSeverity: advisorySeverity,
				}
			}
		}
	}

	return SecurityResult{
		Decision:   DecisionAllow,
		Advisories: advisories,
	}
}

// ScannerFailureResult constructs a SecurityResult for a scan that could not
// complete. The result decision is DecisionScannerFailure, never DecisionAllow.
func ScannerFailureResult(err error) SecurityResult {
	return SecurityResult{
		Decision:    DecisionScannerFailure,
		BlockReason: fmt.Sprintf("security scan failed: %v", err),
	}
}
