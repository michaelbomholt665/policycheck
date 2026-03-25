// internal/tests/policycheck/core/contracts/scope_guard_test.go
package contracts_test

import (
	"testing"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/core/contracts"

	"github.com/stretchr/testify/assert"
)

func TestValidateScopeGuard(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		cfg          config.PolicyConfig
		expectedViol bool
		expectedMsg  string
	}{
		{
			name: "forbidden lifecycle call fails",
			content: `const s = ScopeProjectRepo
func main() {
  os.WriteFile("foo", nil, 0644)
}`,
			cfg:          config.PolicyConfig{ScopeGuard: config.PolicyScopeGuardConfig{Enabled: true, ForbiddenCalls: []string{"os.WriteFile"}}},
			expectedViol: true,
			expectedMsg:  "forbidden lifecycle call found: os.WriteFile",
		},
		{
			name: "disabled scope guard skips violations",
			content: `func main() {
  os.WriteFile("foo", nil, 0644)
}`,
			cfg: config.PolicyConfig{
				ScopeGuard: config.PolicyScopeGuardConfig{
					Enabled:        false,
					ForbiddenCalls: []string{"os.WriteFile"},
				},
			},
			expectedViol: false,
		},
		{
			name: "empty forbidden call list disables those findings",
			content: `const s = ScopeProjectRepo
func main() {
  os.WriteFile("foo", nil, 0644)
}`,
			cfg:          config.PolicyConfig{ScopeGuard: config.PolicyScopeGuardConfig{Enabled: true, ForbiddenCalls: []string{}}},
			expectedViol: false,
		},
		{
			name: "no forbidden calls in content passes",
			content: `func main() {
  fmt.Println("safe")
}`,
			cfg:          config.PolicyConfig{ScopeGuard: config.PolicyScopeGuardConfig{Enabled: true, ForbiddenCalls: []string{"os.WriteFile"}}},
			expectedViol: false,
		},
		{
			name: "go comments and strings do not trigger violations",
			content: `package main

// os.WriteFile is forbidden in lifecycle code.
const forbiddenCall = "os.WriteFile"

func main() {
  fmt.Println(forbiddenCall)
}`,
			cfg:          config.PolicyConfig{ScopeGuard: config.PolicyScopeGuardConfig{Enabled: true, ForbiddenCalls: []string{"os.WriteFile"}}},
			expectedViol: false,
		},
		{
			name: "restrict mode allows configured paths",
			content: `func main() {
  os.WriteFile("foo", nil, 0644)
}`,
			cfg: config.PolicyConfig{
				ScopeGuard: config.PolicyScopeGuardConfig{
					Enabled:             true,
					Mode:                config.ScopeGuardModeRestrict,
					ForbiddenCalls:      []string{"os.WriteFile"},
					AllowedPathPrefixes: []string{"internal/adapters/scanners"},
				},
			},
			expectedViol: false,
		},
		{
			name: "ban mode rejects configured paths",
			content: `func main() {
  os.WriteFile("foo", nil, 0644)
}`,
			cfg: config.PolicyConfig{
				ScopeGuard: config.PolicyScopeGuardConfig{
					Enabled:             true,
					Mode:                config.ScopeGuardModeBan,
					ForbiddenCalls:      []string{"os.WriteFile"},
					AllowedPathPrefixes: []string{"internal/adapters/scanners"},
				},
			},
			expectedViol: true,
			expectedMsg:  "forbidden lifecycle call found: os.WriteFile",
		},
		{
			name: "allow mode disables the rule",
			content: `func main() {
  os.WriteFile("foo", nil, 0644)
}`,
			cfg: config.PolicyConfig{
				ScopeGuard: config.PolicyScopeGuardConfig{
					Enabled:        true,
					Mode:           config.ScopeGuardModeAllow,
					ForbiddenCalls: []string{"os.WriteFile"},
				},
			},
			expectedViol: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			relPath := "main.go"
			if tt.name == "restrict mode allows configured paths" || tt.name == "ban mode rejects configured paths" {
				relPath = "internal/adapters/scanners/extension.go"
			}
			viols, err := contracts.ValidateScopeGuard(relPath, tt.content, tt.cfg)
			assert.NoError(t, err)
			if tt.expectedViol {
				assert.NotEmpty(t, viols)
				assert.Equal(t, "scope-guard", viols[0].RuleID)
				assert.Contains(t, viols[0].Message, tt.expectedMsg)
			} else {
				assert.Empty(t, viols)
			}
		})
	}
}
