// internal/cliwrapper/macro_runner.go
// Executes configured wrapper macros with interpolation and failure-mode handling.
// Keeps macro expansion logic deterministic and testable across command runners.
package cliwrapper

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var macroTemplatePattern = regexp.MustCompile(`\{\{\.(?P<name>[A-Za-z0-9_-]+)\}\}`)

const (
	// MacroOnFailureStop halts the macro at the first failed step.
	MacroOnFailureStop = "stop"
	// MacroOnFailureContinue continues after failed steps and returns an aggregate error.
	MacroOnFailureContinue = "continue"
)

// MacroExecFunc executes one parsed macro step.
type MacroExecFunc func(ctx context.Context, args []string) error

// MacroRunner executes named wrapper macros against an injected command runner.
type MacroRunner struct {
	Macros []WrapperMacroConfig
	Exec   MacroExecFunc
	Vars   map[string]string
}

// RunMacro executes the named macro in step order.
func (r MacroRunner) RunMacro(ctx context.Context, name string) error {
	if r.Exec == nil {
		return fmt.Errorf("run macro %q: exec func is nil", name)
	}

	macro, ok := findMacroByName(r.Macros, name)
	if !ok {
		return fmt.Errorf("run macro %q: macro not found", name)
	}

	mode, err := NormalizeMacroOnFailure(macro.OnFailure)
	if err != nil {
		return fmt.Errorf("run macro %q: %w", name, err)
	}

	var failures []error
	for index, rawStep := range macro.Steps {
		commandText, args, err := prepareMacroStep(rawStep, r.Vars)
		if err != nil {
			return fmt.Errorf("run macro %q step %d %q: %w", name, index+1, rawStep, err)
		}

		if err := r.Exec(ctx, args); err != nil {
			stepErr := fmt.Errorf(
				"run macro %q step %d %q: %w",
				name,
				index+1,
				commandText,
				err,
			)
			if mode == MacroOnFailureStop {
				return stepErr
			}

			failures = append(failures, stepErr)
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("run macro %q: %w", name, errors.Join(failures...))
	}

	return nil
}

// NormalizeMacroOnFailure returns the configured failure mode or the default stop mode.
func NormalizeMacroOnFailure(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case "", MacroOnFailureStop:
		return MacroOnFailureStop, nil
	case MacroOnFailureContinue:
		return MacroOnFailureContinue, nil
	default:
		return "", fmt.Errorf("invalid on_failure %q", value)
	}
}

// InterpolateTemplate replaces {{.name}} placeholders with supplied values.
func InterpolateTemplate(input string, vars map[string]string) (string, error) {
	missing := ""
	output := macroTemplatePattern.ReplaceAllStringFunc(input, func(match string) string {
		submatches := macroTemplatePattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		name := submatches[1]
		value, ok := vars[name]
		if !ok {
			missing = name
			return match
		}

		return value
	})

	if missing != "" {
		return "", fmt.Errorf("missing template variable: %s", missing)
	}

	return output, nil
}

// findMacroByName returns the first configured macro with the requested name.
func findMacroByName(macros []WrapperMacroConfig, name string) (WrapperMacroConfig, bool) {
	for _, macro := range macros {
		if macro.Name == name {
			return macro, true
		}
	}

	return WrapperMacroConfig{}, false
}

// prepareMacroStep expands template variables and tokenizes one macro command step.
func prepareMacroStep(step string, vars map[string]string) (string, []string, error) {
	interpolated, err := InterpolateTemplate(step, vars)
	if err != nil {
		return "", nil, fmt.Errorf("interpolate template: %w", err)
	}

	args, err := splitCommandLine(interpolated)
	if err != nil {
		return "", nil, fmt.Errorf("parse command line: %w", err)
	}

	if len(args) == 0 {
		return "", nil, fmt.Errorf("empty command")
	}

	return interpolated, args, nil
}

type splitState struct {
	current strings.Builder
	quote   rune
	escaped bool
	tokens  []string
}

func (s *splitState) flush() {
	if s.current.Len() > 0 {
		s.tokens = append(s.tokens, s.current.String())
		s.current.Reset()
	}
}

func (s *splitState) processRune(r rune) {
	switch {
	case s.escaped:
		s.current.WriteRune(r)
		s.escaped = false
	case r == '\\':
		s.escaped = true
	case s.quote != 0:
		if r == s.quote {
			s.quote = 0
			return
		}
		s.current.WriteRune(r)
	case r == '\'' || r == '"':
		s.quote = r
	case r == ' ' || r == '\t' || r == '\n':
		s.flush()
	default:
		s.current.WriteRune(r)
	}
}

// splitCommandLine tokenizes one interpolated macro command using shell-like quoting rules.
func splitCommandLine(input string) ([]string, error) {
	state := &splitState{tokens: make([]string, 0)}

	for _, r := range input {
		state.processRune(r)
	}

	if state.escaped {
		return nil, fmt.Errorf("trailing escape")
	}

	if state.quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}

	state.flush()

	return state.tokens, nil
}
