// internal/tests/policycheck/config/validation_test.go
package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"policycheck/internal/policycheck/config"
)

func TestValidatePolicyConfig(t *testing.T) {
	t.Run("Valid Config", func(t *testing.T) {
		cfg := config.PolicyConfig{}
		config.ApplyPolicyConfigDefaults(&cfg)
		// set up required valid state
		cfg.FileSize.MinWarnLOC = 450
		cfg.FileSize.MinMaxLOC = 650
		cfg.FileSize.MinWarnToMaxGap = 150
		cfg.FileSize.WarnLOC = 700
		cfg.FileSize.MaxLOC = 900

		err := config.ValidatePolicyConfig(&cfg)
		assert.NoError(t, err)
	})

	t.Run("Invalid File Size Threshold Gap", func(t *testing.T) {
		cfg := config.PolicyConfig{}
		config.ApplyPolicyConfigDefaults(&cfg)
		cfg.FileSize.MinWarnLOC = 500
		cfg.FileSize.MinMaxLOC = 600
		cfg.FileSize.MinWarnToMaxGap = 150

		err := config.ValidatePolicyConfig(&cfg)
		assert.ErrorContains(t, err, "file_size.min_max_loc (600) must be at least min_warn_loc (500) + min_warn_to_max_gap (150)")
	})

	t.Run("Invalid Custom Rule Regex", func(t *testing.T) {
		cfg := config.PolicyConfig{}
		config.ApplyPolicyConfigDefaults(&cfg)
		cfg.CustomRules = []config.PolicyCustomRule{
			{
				ID:       "bad-regex",
				Pattern:  "(unclosed",
				Severity: "error",
				Enabled:  true,
			},
		}

		err := config.ValidatePolicyConfig(&cfg)
		assert.ErrorContains(t, err, "custom_rules[0] (bad-regex): invalid pattern")
	})

	t.Run("Invalid Custom Rule Severity", func(t *testing.T) {
		cfg := config.PolicyConfig{}
		config.ApplyPolicyConfigDefaults(&cfg)
		cfg.CustomRules = []config.PolicyCustomRule{
			{
				ID:       "bad-severity",
				Pattern:  "good",
				Severity: "fatal",
				Enabled:  true,
			},
		}

		err := config.ValidatePolicyConfig(&cfg)
		assert.ErrorContains(t, err, "custom_rules[0] (bad-severity): invalid severity 'fatal', must be 'warn' or 'error'")
	})

	t.Run("Invalid Scope Guard Mode", func(t *testing.T) {
		cfg := config.PolicyConfig{}
		config.ApplyPolicyConfigDefaults(&cfg)
		cfg.FileSize.MinWarnLOC = 450
		cfg.FileSize.MinMaxLOC = 650
		cfg.FileSize.MinWarnToMaxGap = 150
		cfg.ScopeGuard.Mode = "unsafe"

		err := config.ValidatePolicyConfig(&cfg)
		assert.ErrorContains(t, err, `scope_guard: invalid mode "unsafe"`)
	})

	t.Run("Invalid Scope Guard Absolute Prefix", func(t *testing.T) {
		cfg := config.PolicyConfig{}
		config.ApplyPolicyConfigDefaults(&cfg)
		cfg.FileSize.MinWarnLOC = 450
		cfg.FileSize.MinMaxLOC = 650
		cfg.FileSize.MinWarnToMaxGap = 150
		cfg.ScopeGuard.AllowedPathPrefixes = []string{`C:\Windows\System32`}

		err := config.ValidatePolicyConfig(&cfg)
		assert.ErrorContains(t, err, `scope_guard: allowed_path_prefixes must be repo-relative`)
	})

	t.Run("Invalid Scope Guard Traversal Prefix", func(t *testing.T) {
		cfg := config.PolicyConfig{}
		config.ApplyPolicyConfigDefaults(&cfg)
		cfg.FileSize.MinWarnLOC = 450
		cfg.FileSize.MinMaxLOC = 650
		cfg.FileSize.MinWarnToMaxGap = 150
		cfg.ScopeGuard.AllowedPathPrefixes = []string{"../internal/adapters/scanners"}

		err := config.ValidatePolicyConfig(&cfg)
		assert.ErrorContains(t, err, `scope_guard: allowed_path_prefixes must stay within the repository`)
	})

	t.Run("Valid Scope Guard Prefix Is Normalized", func(t *testing.T) {
		cfg := config.PolicyConfig{}
		config.ApplyPolicyConfigDefaults(&cfg)
		cfg.FileSize.MinWarnLOC = 450
		cfg.FileSize.MinMaxLOC = 650
		cfg.FileSize.MinWarnToMaxGap = 150
		cfg.ScopeGuard.AllowedPathPrefixes = []string{"./internal/adapters/scanners/"}

		err := config.ValidatePolicyConfig(&cfg)
		assert.NoError(t, err)
		assert.Equal(t, []string{"internal/adapters/scanners"}, cfg.ScopeGuard.AllowedPathPrefixes)
	})
}
