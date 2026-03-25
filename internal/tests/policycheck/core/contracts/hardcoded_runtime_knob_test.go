package contracts_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"policycheck/internal/policycheck/core/contracts"
)

func TestValidateHardcodedRuntimeKnobs(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		identifiers []string
		wantCount   int
	}{
		{
			name: "flags direct variable assignment",
			content: `package sample
var ReadTimeout = 30
`,
			identifiers: []string{"Timeout"},
			wantCount:   1,
		},
		{
			name: "flags struct field literal",
			content: `package sample
var cfg = struct {
	Port int
}{
	Port: 8080,
}
`,
			identifiers: []string{"Port"},
			wantCount:   1,
		},
		{
			name: "flags duration expression",
			content: `package sample
import "time"
var ReadTimeout = 30 * time.Second
`,
			identifiers: []string{"Timeout"},
			wantCount:   1,
		},
		{
			name: "ignores non-matching identifiers",
			content: `package sample
var BatchSize = 30
`,
			identifiers: []string{"Timeout"},
			wantCount:   0,
		},
		{
			name: "ignores non-literal assignments",
			content: `package sample
func loadTimeout() int { return 30 }
var ReadTimeout = loadTimeout()
`,
			identifiers: []string{"Timeout"},
			wantCount:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := contracts.ValidateHardcodedRuntimeKnobs("internal/sample/config.go", []byte(tt.content), tt.identifiers)
			assert.Len(t, violations, tt.wantCount)
			if tt.wantCount > 0 {
				assert.Equal(t, "hardcoded-runtime-knob", violations[0].RuleID)
			}
		})
	}
}
