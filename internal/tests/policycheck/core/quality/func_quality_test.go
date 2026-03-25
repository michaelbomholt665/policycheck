// internal/tests/policycheck/core/quality/func_quality_test.go
package quality_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/core/quality"
	"policycheck/internal/policycheck/types"
)

func TestEvaluateFunctionQualityFacts(t *testing.T) {
	cfg := config.PolicyConfig{
		FunctionQuality: config.PolicyFunctionQualityConfig{
			EnabledLanguages:        []string{"go", "python"},
			WarnLOC:                 80,
			MaxLOC:                  120,
			GoWarnLOC:               80,
			GoMaxLOC:                120,
			PythonWarnLOC:           100,
			PythonMaxLOC:            150,
			TypeScriptWarnLOC:       100,
			TypeScriptMaxLOC:        160,
			WarnParameterCount:      5,
			MaxParameterCount:       7,
			MildCTXMin:              12,
			ElevatedCTXMin:          14,
			ImmediateRefactorCTXMin: 16,
			ErrorCTXMin:             18,
			ErrorCTXAndLOCCTX:       10,
			ErrorCTXAndLOCLOC:       80,
			NilGuardRepeatWarnCount: 8,
		},
	}

	tests := []struct {
		name      string
		fact      types.PolicyFact
		wantCount int
		wantSev   string
		wantMsg   string
	}{
		{
			name: "pass - clean function",
			fact: types.PolicyFact{
				Language:   "go",
				SymbolKind: "function",
				SymbolName: "cleanFunc",
				LineNumber: 10,
				EndLine:    20, // 11 lines
				Complexity: 5,
			},
			wantCount: 0,
		},
		{
			name: "mild complexity warning",
			fact: types.PolicyFact{
				Language:   "go",
				SymbolKind: "function",
				SymbolName: "mildFunc",
				LineNumber: 10,
				EndLine:    20,
				Complexity: 12,
			},
			wantCount: 1,
			wantSev:   "warn",
			wantMsg:   "mild complexity",
		},
		{
			name: "elevated complexity warning",
			fact: types.PolicyFact{
				Language:   "go",
				SymbolKind: "function",
				SymbolName: "elevatedFunc",
				LineNumber: 10,
				EndLine:    20,
				Complexity: 14,
			},
			wantCount: 1,
			wantSev:   "warn",
			wantMsg:   "elevated complexity",
		},
		{
			name: "immediate refactor warning",
			fact: types.PolicyFact{
				Language:   "go",
				SymbolKind: "function",
				SymbolName: "refactorFunc",
				LineNumber: 10,
				EndLine:    20,
				Complexity: 16,
			},
			wantCount: 1,
			wantSev:   "warn",
			wantMsg:   "immediate refactoring",
		},
		{
			name: "hard ctx error",
			fact: types.PolicyFact{
				Language:   "go",
				SymbolKind: "function",
				SymbolName: "errorFunc",
				LineNumber: 10,
				EndLine:    20,
				Complexity: 18,
			},
			wantCount: 1,
			wantSev:   "error",
			wantMsg:   "excessively complex",
		},
		{
			name: "hard loc error",
			fact: types.PolicyFact{
				Language:   "go",
				SymbolKind: "function",
				SymbolName: "longFunc",
				LineNumber: 1,
				EndLine:    120, // 120 lines
				Complexity: 5,
			},
			wantCount: 1,
			wantSev:   "error",
			wantMsg:   "excessively long",
		},
		{
			name: "combined ctx and loc error",
			fact: types.PolicyFact{
				Language:   "go",
				SymbolKind: "function",
				SymbolName: "complexLongFunc",
				LineNumber: 1,
				EndLine:    80, // 80 lines
				Complexity: 10,
			},
			wantCount: 1,
			wantSev:   "error",
			wantMsg:   "both complex and long",
		},
		{
			name: "pass - combined just under",
			fact: types.PolicyFact{
				Language:   "go",
				SymbolKind: "function",
				SymbolName: "almostComplexLongFunc",
				LineNumber: 1,
				EndLine:    79, // 79 lines
				Complexity: 10,
			},
			wantCount: 0,
		},
		{
			name: "disabled language skipped",
			fact: types.PolicyFact{
				Language:   "typescript",
				SymbolKind: "function",
				SymbolName: "tsFunc",
				Complexity: 20,
			},
			wantCount: 0,
		},
		{
			name: "non-function skipped",
			fact: types.PolicyFact{
				Language:   "go",
				SymbolKind: "struct",
				SymbolName: "MyStruct",
				Complexity: 20,
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viols := quality.EvaluateFunctionQualityFacts([]types.PolicyFact{tt.fact}, cfg)
			assert.Len(t, viols, tt.wantCount)
			if tt.wantCount > 0 {
				assert.Equal(t, tt.wantSev, viols[0].Severity)
				assert.Contains(t, viols[0].Message, tt.wantMsg)
			}
		})
	}
}
