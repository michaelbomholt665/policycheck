// internal/tests/policycheck/core/contracts/go_version_test.go
package contracts_test

import (
	"testing"

	"policycheck/internal/policycheck/core/contracts"

	"github.com/stretchr/testify/assert"
)

func TestValidateGoVersion(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		allowedPrefixes []string
		expectedErr     bool
		expectedViol    bool
	}{
		{
			name:            "allowed prefix passes 1.24",
			content:         "module foo\n\ngo 1.24.0\ntoolchain go1.24.0\n",
			allowedPrefixes: []string{"1.24", "1.25"},
			expectedErr:     false,
			expectedViol:    false,
		},
		{
			name:            "allowed prefix passes 1.25",
			content:         "module foo\n\ngo 1.25.3\ntoolchain go1.25.3\n",
			allowedPrefixes: []string{"1.24", "1.25"},
			expectedErr:     false,
			expectedViol:    false,
		},
		{
			name:            "disallowed prefix fails",
			content:         "module foo\n\ngo 1.23.1\ntoolchain go1.23.1\n",
			allowedPrefixes: []string{"1.24", "1.25"},
			expectedErr:     false,
			expectedViol:    true,
		},
		{
			name:            "go 1.26.0 with default config fails",
			content:         "module foo\n\ngo 1.26.0\ntoolchain go1.26.0\n",
			allowedPrefixes: []string{"1.24", "1.25"},
			expectedErr:     false,
			expectedViol:    true,
		},
		{
			name:            "go 1.26.0 with override config passes",
			content:         "module foo\n\ngo 1.26.0\ntoolchain go1.26.0\n",
			allowedPrefixes: []string{"1.26"},
			expectedErr:     false,
			expectedViol:    false,
		},
		{
			name:            "missing go directive fails",
			content:         "module foo\n\nrequire github.com/foo/bar v1.0.0\n",
			allowedPrefixes: []string{"1.24", "1.25"},
			expectedErr:     false,
			expectedViol:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viols, _, err := contracts.ValidateGoVersion(tt.content, tt.allowedPrefixes)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.expectedViol {
					assert.NotEmpty(t, viols)
					assert.Equal(t, "go-version", viols[0].RuleID)
				} else {
					assert.Empty(t, viols)
				}
			}
		})
	}
}
