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
			cfg:          config.PolicyConfig{ScopeGuard: config.PolicyScopeGuardConfig{ForbiddenCalls: []string{"os.WriteFile"}}},
			expectedViol: true,
			expectedMsg:  "forbidden lifecycle call found: os.WriteFile",
		},
		{
			name: "empty forbidden call list disables those findings",
			content: `const s = ScopeProjectRepo
func main() {
  os.WriteFile("foo", nil, 0644)
}`,
			cfg:          config.PolicyConfig{ScopeGuard: config.PolicyScopeGuardConfig{ForbiddenCalls: []string{}}},
			expectedViol: false,
		},
		{
			name: "no forbidden calls in content passes",
			content: `func main() {
  fmt.Println("safe")
}`,
			cfg:          config.PolicyConfig{ScopeGuard: config.PolicyScopeGuardConfig{ForbiddenCalls: []string{"os.WriteFile"}}},
			expectedViol: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viols, err := contracts.ValidateScopeGuard("main.go", tt.content, tt.cfg)
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
