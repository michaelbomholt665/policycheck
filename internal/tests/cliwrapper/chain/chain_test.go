// internal/tests/cliwrapper/chain/chain_test.go
package chain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"policycheck/internal/cliwrapper"
)

// execRecorder is a test double that records calls and controls return values.
type execRecorder struct {
	calls  [][]string
	errors map[string]error // keyed by args[0]
}

func (r *execRecorder) exec(ctx context.Context, args []string) error {
	r.calls = append(r.calls, args)
	if r.errors != nil {
		if err, ok := r.errors[args[0]]; ok {
			return err
		}
	}

	return nil
}

// TestSplitChain_NoThen verifies that args without -then produce chained=false.
func TestSplitChain_NoThen(t *testing.T) {
	t.Parallel()

	gate, main, chained := cliwrapper.SplitChain([]string{"go", "build", "./..."})
	assert.False(t, chained)
	assert.Empty(t, gate)
	assert.Equal(t, []string{"go", "build", "./..."}, main)
}

// TestSplitChain_WithThen verifies correct split at the -then token.
func TestSplitChain_WithThen(t *testing.T) {
	t.Parallel()

	gate, main, chained := cliwrapper.SplitChain([]string{"go", "build", "-then", "go", "test"})
	assert.True(t, chained)
	assert.Equal(t, []string{"go", "build"}, gate)
	assert.Equal(t, []string{"go", "test"}, main)
}

// TestRunChain_GatePassesMainRuns verifies that main executes when gate succeeds.
func TestRunChain_GatePassesMainRuns(t *testing.T) {
	t.Parallel()

	rec := &execRecorder{}
	gate := []string{"go", "build"}
	main := []string{"go", "test"}

	err := cliwrapper.RunChain(context.Background(), gate, main, rec.exec)
	require.NoError(t, err)
	require.Len(t, rec.calls, 2)
	assert.Equal(t, gate, rec.calls[0])
	assert.Equal(t, main, rec.calls[1])
}

// TestRunChain_GateFailsMainSkipped verifies that main is not executed when gate fails.
func TestRunChain_GateFailsMainSkipped(t *testing.T) {
	t.Parallel()

	gateErr := errors.New("build failed")
	rec := &execRecorder{
		errors: map[string]error{"go": gateErr},
	}

	gate := []string{"go", "build"}
	main := []string{"go", "test"}

	err := cliwrapper.RunChain(context.Background(), gate, main, rec.exec)
	require.Error(t, err)
	assert.ErrorIs(t, err, gateErr)
	// Main must not have been called.
	require.Len(t, rec.calls, 1)
}

// TestRunChain_ContextCancelledBeforeGate verifies cancelled context is propagated.
func TestRunChain_ContextCancelledBeforeGate(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	rec := &execRecorder{}
	err := cliwrapper.RunChain(ctx, []string{"go", "build"}, []string{"go", "test"}, rec.exec)
	require.Error(t, err)
	assert.Empty(t, rec.calls, "exec must not be called on cancelled context")
}
