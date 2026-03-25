// internal/tests/policycheck/core/quality/file_size_test.go
package quality_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"policycheck/internal/policycheck/config"
	"policycheck/internal/policycheck/core/quality"
)

func TestComputeFileSizeThresholds(t *testing.T) {
	cfg := config.PolicyFileSizeConfig{
		WarnLOC:                   700,
		MaxLOC:                    900,
		MinWarnLOC:                450,
		MinMaxLOC:                 650,
		MinWarnToMaxGap:           150,
		WarnPenaltyPerCTXFunction: 10,
		MaxPenaltyPerCTXFunction:  15,
	}

	tests := []struct {
		name             string
		warnCtxFuncCount int
		hardCtxFuncCount int
		wantWarn         int
		wantMax          int
	}{
		{
			name:             "no ctx penalty",
			warnCtxFuncCount: 0,
			hardCtxFuncCount: 0,
			wantWarn:         700,
			wantMax:          900,
		},
		{
			name:             "moderate ctx penalty (5 functions)",
			warnCtxFuncCount: 5,
			hardCtxFuncCount: 5,
			wantWarn:         650,
			wantMax:          825,
		},
		{
			name:             "high ctx penalty reaching floors",
			warnCtxFuncCount: 30,
			hardCtxFuncCount: 30, // 700 - 300 = 400 (below 450 floor); 900 - 450 = 450 (below 650 floor)
			wantWarn:         450,
			wantMax:          650,
		},
		{
			name:             "gap enforcement",
			warnCtxFuncCount: 22,
			hardCtxFuncCount: 22, // warn: 700-220=480; max: 900-330=570. Max floor is 650. Gap 650-480=170. Gap min is 150. Max remains 650.
			wantWarn:         480,
			wantMax:          650,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warn, max := quality.ComputeFileSizeThresholds(cfg, tt.warnCtxFuncCount, tt.hardCtxFuncCount)
			assert.Equal(t, tt.wantWarn, warn, "warn threshold")
			assert.Equal(t, tt.wantMax, max, "max threshold")
		})
	}
}

func TestEvaluateFileSize(t *testing.T) {
	tests := []struct {
		name      string
		lineCount int
		warn      int
		max       int
		wantCount int
		wantSev   string
	}{
		{
			name:      "pass",
			lineCount: 500,
			warn:      600,
			max:       800,
			wantCount: 0,
		},
		{
			name:      "warn boundary",
			lineCount: 601,
			warn:      600,
			max:       800,
			wantCount: 1,
			wantSev:   "warn",
		},
		{
			name:      "max boundary",
			lineCount: 801,
			warn:      600,
			max:       800,
			wantCount: 1,
			wantSev:   "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viols := quality.EvaluateFileSize("foo.go", tt.lineCount, tt.warn, tt.max)
			assert.Len(t, viols, tt.wantCount)
			if tt.wantCount > 0 {
				assert.Equal(t, tt.wantSev, viols[0].Severity)
				assert.Contains(t, viols[0].Message, "foo.go")
			}
		})
	}
}
