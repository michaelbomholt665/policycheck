// internal/tests/policycheck/core/custom/custom_rules_test.go
package custom_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/core/custom"
)

func TestCheckContentForPattern(t *testing.T) {
	rule := config.PolicyCustomRule{
		ID:              "no-db-query",
		Message:         "Avoid direct DB queries",
		Severity:        "error",
		CompiledPattern: regexp.MustCompile(`db\.Query`),
	}

	content := `package main
func main() {
    db.Query("SELECT 1")
    fmt.Println("Hello")
    db.Query("SELECT 2")
}`

	violations := custom.CheckContentForPattern("main.go", content, rule)

	assert.Len(t, violations, 2)
	assert.Equal(t, 3, violations[0].Line)
	assert.Equal(t, 5, violations[1].Line)
	assert.Equal(t, "custom.no-db-query", violations[0].RuleID)
}

func TestMatchesRuleCriteria(t *testing.T) {
	tests := []struct {
		name     string
		rel      string
		path     string
		rule     config.PolicyCustomRule
		expected bool
	}{
		{
			name:     "Match language go",
			rel:      "main.go",
			path:     "/root/main.go",
			rule:     config.PolicyCustomRule{Language: "go"},
			expected: true,
		},
		{
			name:     "Mismatch language go",
			rel:      "main.py",
			path:     "/root/main.py",
			rule:     config.PolicyCustomRule{Language: "go"},
			expected: false,
		},
		{
			name:     "Match language any",
			rel:      "main.py",
			path:     "/root/main.py",
			rule:     config.PolicyCustomRule{Language: "any"},
			expected: true,
		},
		{
			name:     "Match glob",
			rel:      "internal/app/main.go",
			path:     "/root/internal/app/main.go",
			rule:     config.PolicyCustomRule{FileGlob: "internal/app/*.go"},
			expected: true,
		},
		{
			name:     "Match recursive glob",
			rel:      "internal/app/sub/main.go",
			path:     "/root/internal/app/sub/main.go",
			rule:     config.PolicyCustomRule{FileGlob: "internal/**/*.go"},
			expected: true,
		},
		{
			name:     "Mismatch glob",
			rel:      "cmd/main.go",
			path:     "/root/cmd/main.go",
			rule:     config.PolicyCustomRule{FileGlob: "internal/**/*.go"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, custom.MatchesRuleCriteria(tt.rel, tt.path, tt.rule))
		})
	}
}
