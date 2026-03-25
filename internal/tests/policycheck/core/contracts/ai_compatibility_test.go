// internal/tests/policycheck/core/contracts/ai_compatibility_test.go
package contracts_test

import (
	"testing"

	"policycheck/internal/policycheck/core/contracts"

	"github.com/stretchr/testify/assert"
)

func TestValidateAICompatibility(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		requiredFlags []string
		expectedViol  bool
		isWrapper     bool
	}{
		{
			name:          "both required flags present passes",
			content:       `var flags = []string{"--ai", "--user"}`,
			requiredFlags: []string{"--ai", "--user"},
			expectedViol:  false,
			isWrapper:     false,
		},
		{
			name:          "missing one required flag fails",
			content:       `var flags = []string{"--ai"}`,
			requiredFlags: []string{"--ai", "--user"},
			expectedViol:  true,
			isWrapper:     false,
		},
		{
			name:          "wrapper resolution continues past thin wrapper file",
			content:       `package main\nfunc main() {\nRunCLI()\n}`,
			requiredFlags: []string{"--ai", "--user"},
			expectedViol:  false,
			isWrapper:     true,
		},
		{
			name:          "override test only one flag",
			content:       `var flags = []string{"--ai"}`,
			requiredFlags: []string{"--ai"},
			expectedViol:  false,
			isWrapper:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isWrapper, viols, err := contracts.ValidateAICompatibility("main.go", tt.content, tt.requiredFlags)
			assert.NoError(t, err)
			assert.Equal(t, tt.isWrapper, isWrapper)
			if tt.expectedViol {
				assert.NotEmpty(t, viols)
				assert.Equal(t, "ai-compatibility", viols[0].RuleID)
			} else {
				assert.Empty(t, viols)
			}
		})
	}
}
