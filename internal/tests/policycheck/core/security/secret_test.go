// internal/tests/policycheck/core/security/secret_test.go
package security_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/core/security"
	"policycheck/internal/policycheck/types"
)

func TestIsBenignSecretExample(t *testing.T) {
	hints := []string{"example", "sample"}
	assert.True(t, security.IsBenignSecretExample("this is an example", hints))
	assert.True(t, security.IsBenignSecretExample("this is a SAMPLE", hints))
	assert.False(t, security.IsBenignSecretExample("this is a real secret", hints))
}

func TestIsObviousPlaceholderSecret(t *testing.T) {
	placeholders := []string{"<token>", "changeme"}
	assert.True(t, security.IsObviousPlaceholderSecret("use <token> here", placeholders))
	assert.True(t, security.IsObviousPlaceholderSecret("value is changeme", placeholders))
	// Test regex placeholder
	assert.True(t, security.IsObviousPlaceholderSecret("ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", nil))
	assert.False(t, security.IsObviousPlaceholderSecret("ghp_realtoken12345678901234567890123456", nil))
}

func TestSecretSeverityRank(t *testing.T) {
	assert.Equal(t, 4, security.SecretSeverityRank("CRITICAL"))
	assert.Equal(t, 3, security.SecretSeverityRank("high"))
	assert.Equal(t, 2, security.SecretSeverityRank("Medium"))
	assert.Equal(t, 1, security.SecretSeverityRank("LOW"))
	assert.Equal(t, -1, security.SecretSeverityRank("OFF"))
	assert.Equal(t, 0, security.SecretSeverityRank("unknown"))
}

func TestPickBestSecretFinding(t *testing.T) {
	viols := []types.Violation{
		{RuleID: "generic_api_key_assignment", Severity: "MEDIUM"},
		{RuleID: "github_pat_classic", Severity: "CRITICAL"},
		{RuleID: "secret-keyword", Severity: "LOW"},
	}

	best := security.PickBestSecretFinding(viols)
	assert.Equal(t, "github_pat_classic", best.RuleID)
	assert.Equal(t, "CRITICAL", best.Severity)

	// Test specific vs generic
	viols2 := []types.Violation{
		{RuleID: "generic_api_key_assignment", Severity: "MEDIUM"},
		{RuleID: "aws_access_key_id", Severity: "HIGH"},
	}
	best2 := security.PickBestSecretFinding(viols2)
	assert.Equal(t, "aws_access_key_id", best2.RuleID)
}

func TestFilterAllowlistedSecretFindings(t *testing.T) {
	cfg := config.PolicySecretLoggingConfig{
		Allowlist: config.PolicySecretLoggingAllowlist{
			PatternIDs: []string{"github_pat_classic"},
		},
		Overrides: map[string]string{
			"aws_access_key_id":          "OFF",
			"generic_api_key_assignment": "CRITICAL",
		},
	}

	viols := []types.Violation{
		{RuleID: "github_pat_classic", Severity: "CRITICAL"},
		{RuleID: "aws_access_key_id", Severity: "HIGH"},
		{RuleID: "generic_api_key_assignment", Severity: "MEDIUM"},
		{RuleID: "something_else", Severity: "LOW"},
	}

	filtered := security.FilterAllowlistedSecretFindings(viols, cfg)
	assert.Len(t, filtered, 2)
	assert.Equal(t, "generic_api_key_assignment", filtered[0].RuleID)
	assert.Equal(t, "CRITICAL", filtered[0].Severity)
	assert.Equal(t, "something_else", filtered[1].RuleID)
}

func TestValidatePolicyConfig_CompilesAllowlistLiteralPatterns(t *testing.T) {
	cfg := config.PolicyConfig{}
	config.ApplyPolicyConfigDefaults(&cfg)
	cfg.FileSize.MinWarnLOC = 450
	cfg.FileSize.MinMaxLOC = 650
	cfg.FileSize.MinWarnToMaxGap = 150
	cfg.SecretLogging.Allowlist.LiteralPatterns = []string{`ghp_[a-z0-9]+`}

	err := config.ValidatePolicyConfig(&cfg)

	assert.NoError(t, err)
	assert.Len(t, cfg.SecretLogging.CompiledAllowedLiteralPatterns, 1)
	assert.True(t, security.IsAllowedLiteral("ghp_abcdef123456", cfg.SecretLogging.CompiledAllowedLiteralPatterns))
}

func TestScanContentForSecrets(t *testing.T) {
	cfg := config.PolicySecretLoggingConfig{
		Keywords:           []string{"password"},
		BenignHints:        []string{"example"},
		PlaceholderStrings: []string{"changeme"},
	}
	patterns := security.BuiltInPatterns()

	content := `
		// This is a test file
		apiKey := "ghp_123456789012345678901234567890123456"
		password := "mysecret"
		examplePassword := "example"
		placeholder := "changeme"
	`

	viols := security.ScanContentForSecrets("test.go", content, patterns, cfg)

	// Expecting 2 total violations
	assert.Len(t, viols, 2)

	foundGitHub := false
	foundPasswordLine := false
	for _, v := range viols {
		if v.RuleID == "github_pat_classic" {
			foundGitHub = true
		}
		if v.RuleID == "secret-keyword" || v.RuleID == "standalone_password_candidate" {
			foundPasswordLine = true
		}
	}
	assert.True(t, foundGitHub)
	assert.True(t, foundPasswordLine)
}
