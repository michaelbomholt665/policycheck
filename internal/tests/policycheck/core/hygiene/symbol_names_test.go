// internal/tests/policycheck/core/hygiene/symbol_names_test.go
package hygiene_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"policycheck/internal/policycheck/core/hygiene"
)

func TestCountTokens(t *testing.T) {
	tests := []struct {
		name     string
		expected int
	}{
		{"ValidateSchema", 2},
		{"parseGoAST", 3},
		{"HTTPHandler", 2},
		{"validate", 1},
		{"v", 1},
		{"XMLParser", 2},
		{"NewJSONEncoder", 3},
		{"main", 2}, // Special case
		{"my_symbol", 2},
		{"My_Symbol", 2},
		{"DB", 1},
		{"UserByID", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, hygiene.CountTokens(tt.name))
		})
	}
}
