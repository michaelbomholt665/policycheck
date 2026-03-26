// internal/tests/cliwrapper/security/security_gate_test.go
package security_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"policycheck/internal/cliwrapper"
)

// TestEvaluateSeverity_Allow verifies that no advisories always allows.
func TestEvaluateSeverity_Allow(t *testing.T) {
	t.Parallel()

	result := cliwrapper.EvaluateSeverity(cliwrapper.SeverityHigh, nil)
	assert.Equal(t, cliwrapper.DecisionAllow, result.Decision)
	assert.Empty(t, result.Advisories)
	assert.Empty(t, result.BlockReason)
}

// TestEvaluateSeverity_Block verifies that a critical advisory blocks when
// threshold is high.
func TestEvaluateSeverity_Block(t *testing.T) {
	t.Parallel()

	advisories := []cliwrapper.Advisory{
		{ID: "GHSA-xxxx-yyyy-zzzz", Summary: "RCE via prototype pollution", Severity: "critical"},
	}

	result := cliwrapper.EvaluateSeverity(cliwrapper.SeverityHigh, advisories)
	assert.Equal(t, cliwrapper.DecisionBlock, result.Decision)
	assert.NotEmpty(t, result.BlockReason)
}

// TestEvaluateSeverity_BelowThreshold verifies that a low advisory is allowed
// when the threshold is high.
func TestEvaluateSeverity_BelowThreshold(t *testing.T) {
	t.Parallel()

	advisories := []cliwrapper.Advisory{
		{ID: "CVE-2024-0001", Summary: "minor issue", Severity: "low"},
	}

	result := cliwrapper.EvaluateSeverity(cliwrapper.SeverityHigh, advisories)
	assert.Equal(t, cliwrapper.DecisionAllow, result.Decision)
}

// TestEvaluateSeverity_ScannerFailure verifies that a scanner-failure decision
// is represented correctly and is distinct from block.
func TestEvaluateSeverity_ScannerFailure(t *testing.T) {
	t.Parallel()

	err := errors.New("OSV unreachable")
	result := cliwrapper.ScannerFailureResult(err)

	assert.Equal(t, cliwrapper.DecisionScannerFailure, result.Decision)
	assert.NotEmpty(t, result.BlockReason)
}

// TestEvaluateSeverity_NeverDowngradesBlock verifies that scanner failure is
// never silently treated as allow.
func TestEvaluateSeverity_NeverDowngradesBlock(t *testing.T) {
	t.Parallel()

	err := errors.New("timeout")
	result := cliwrapper.ScannerFailureResult(err)

	assert.NotEqual(t, cliwrapper.DecisionAllow, result.Decision,
		"scanner failure must never produce DecisionAllow")
}
