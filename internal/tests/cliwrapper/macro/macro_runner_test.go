package macro_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"policycheck/internal/cliwrapper"
)

func TestInterpolateTemplate_ReplacesProvidedVariables(t *testing.T) {
	t.Parallel()

	got, err := cliwrapper.InterpolateTemplate(`git commit -m "{{.message}}"`, map[string]string{
		"message": "fix: typo",
	})

	require.NoError(t, err)
	assert.Equal(t, `git commit -m "fix: typo"`, got)
}

func TestMacroRunner_MissingTemplateVariable_ReturnsErrorBeforeExec(t *testing.T) {
	t.Parallel()

	called := false
	runner := cliwrapper.MacroRunner{
		Macros: []cliwrapper.WrapperMacroConfig{
			{Name: "commit", Steps: []string{`git commit -m "{{.message}}"`}},
		},
		Exec: func(_ context.Context, _ []string) error {
			called = true
			return nil
		},
	}

	err := runner.RunMacro(context.Background(), "commit")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing template variable: message")
	assert.False(t, called)
}

func TestMacroRunner_StopOnFailure_HaltsAtFirstFailedStep(t *testing.T) {
	t.Parallel()

	failure := errors.New("exit 1")
	var calls [][]string
	runner := cliwrapper.MacroRunner{
		Macros: []cliwrapper.WrapperMacroConfig{
			{
				Name:      "ci",
				Steps:     []string{"go test ./...", "go vet ./..."},
				OnFailure: cliwrapper.MacroOnFailureStop,
			},
		},
		Exec: func(_ context.Context, args []string) error {
			calls = append(calls, append([]string(nil), args...))
			return failure
		},
	}

	err := runner.RunMacro(context.Background(), "ci")

	require.Error(t, err)
	assert.ErrorIs(t, err, failure)
	assert.Contains(t, err.Error(), `"go test ./..."`)
	require.Len(t, calls, 1)
	assert.Equal(t, []string{"go", "test", "./..."}, calls[0])
}

func TestMacroRunner_ContinueOnFailure_ReturnsAggregateError(t *testing.T) {
	t.Parallel()

	first := errors.New("first failure")
	second := errors.New("second failure")
	var calls [][]string
	index := 0
	runner := cliwrapper.MacroRunner{
		Macros: []cliwrapper.WrapperMacroConfig{
			{
				Name:      "release",
				Steps:     []string{"go test ./...", "go vet ./...", "go build ./..."},
				OnFailure: cliwrapper.MacroOnFailureContinue,
			},
		},
		Exec: func(_ context.Context, args []string) error {
			calls = append(calls, append([]string(nil), args...))
			index++
			switch index {
			case 1:
				return first
			case 2:
				return second
			default:
				return nil
			}
		},
	}

	err := runner.RunMacro(context.Background(), "release")

	require.Error(t, err)
	assert.ErrorIs(t, err, first)
	assert.ErrorIs(t, err, second)
	assert.Contains(t, err.Error(), `"go test ./..."`)
	assert.Contains(t, err.Error(), `"go vet ./..."`)
	require.Len(t, calls, 3)
}
