package cliwrapper

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

var macroTemplatePattern = regexp.MustCompile(`\{\{\.(?P<name>[A-Za-z0-9_-]+)\}\}`)

// MacroRunnerAdapter loads wrapper config and executes the named macro.
type MacroRunnerAdapter struct {
	loadConfig func() (WrapperConfig, error)
	exec       ExecFunc
	vars       map[string]string
}

// NewMacroRunnerAdapter returns a macro runner with production defaults.
func NewMacroRunnerAdapter() *MacroRunnerAdapter {
	return NewMacroRunnerAdapterWithDeps(loadActiveAdapterConfig, defaultExecFunc, nil)
}

// NewMacroRunnerAdapterWithDeps returns a macro runner with injected seams for tests.
func NewMacroRunnerAdapterWithDeps(
	loadConfig func() (WrapperConfig, error),
	exec ExecFunc,
	vars map[string]string,
) *MacroRunnerAdapter {
	return &MacroRunnerAdapter{
		loadConfig: loadConfig,
		exec:       exec,
		vars:       vars,
	}
}

// RunMacro loads the active wrapper config and executes the named macro.
func (a *MacroRunnerAdapter) RunMacro(ctx context.Context, name string) error {
	cfg, err := a.loadConfig()
	if err != nil {
		return fmt.Errorf("macro runner adapter: load config: %w", err)
	}

	runner := adapterMacroRunner{
		macros: cfg.Macros,
		exec:   a.exec,
		vars:   a.vars,
	}

	if err := runner.runMacro(ctx, name); err != nil {
		return fmt.Errorf("macro runner adapter: %w", err)
	}

	return nil
}

func loadActiveAdapterConfig() (WrapperConfig, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return WrapperConfig{}, fmt.Errorf("get working directory: %w", err)
	}

	configPath, err := findActiveWrapperConfigPath(workingDir)
	if err != nil {
		return WrapperConfig{}, fmt.Errorf("find wrapper config: %w", err)
	}

	if configPath == "" {
		return WrapperConfig{}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return WrapperConfig{}, fmt.Errorf("read %s: %w", configPath, err)
	}

	var cfg WrapperConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return WrapperConfig{}, fmt.Errorf("parse %s: %w", configPath, err)
	}

	return cfg, nil
}

func findActiveWrapperConfigPath(startDir string) (string, error) {
	current := filepath.Clean(startDir)

	for {
		for _, name := range []string{"wrapper-gate.toml", "policy-gate.toml"} {
			candidate := filepath.Join(current, name)
			info, err := os.Stat(candidate)
			if err == nil && !info.IsDir() {
				return candidate, nil
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", nil
		}

		current = parent
	}
}

type adapterMacroRunner struct {
	macros []WrapperMacroConfig
	exec   ExecFunc
	vars   map[string]string
}

func (r adapterMacroRunner) runMacro(ctx context.Context, name string) error {
	macro, ok := findMacroByName(r.macros, name)
	if !ok {
		return fmt.Errorf("run macro %q: macro not found", name)
	}

	mode, err := normalizeMacroOnFailure(macro.OnFailure)
	if err != nil {
		return fmt.Errorf("run macro %q: %w", name, err)
	}

	failures := make([]error, 0)
	for index, step := range macro.Steps {
		commandText, args, err := prepareAdapterMacroStep(step, r.vars)
		if err != nil {
			return fmt.Errorf("run macro %q step %d %q: %w", name, index+1, step, err)
		}

		if err := r.exec(ctx, args); err != nil {
			stepErr := fmt.Errorf("run macro %q step %d %q: %w", name, index+1, commandText, err)
			if mode == "stop" {
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

func findMacroByName(macros []WrapperMacroConfig, name string) (WrapperMacroConfig, bool) {
	for _, macro := range macros {
		if macro.Name == name {
			return macro, true
		}
	}

	return WrapperMacroConfig{}, false
}

func normalizeMacroOnFailure(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case "", "stop":
		return "stop", nil
	case "continue":
		return "continue", nil
	default:
		return "", fmt.Errorf("invalid on_failure %q", value)
	}
}

func prepareAdapterMacroStep(step string, vars map[string]string) (string, []string, error) {
	interpolated, err := interpolateMacroTemplate(step, vars)
	if err != nil {
		return "", nil, fmt.Errorf("interpolate template: %w", err)
	}

	args, err := splitMacroCommandLine(interpolated)
	if err != nil {
		return "", nil, fmt.Errorf("parse command line: %w", err)
	}

	if len(args) == 0 {
		return "", nil, fmt.Errorf("empty command")
	}

	return interpolated, args, nil
}

func interpolateMacroTemplate(input string, vars map[string]string) (string, error) {
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

func splitMacroCommandLine(input string) ([]string, error) {
	tokens := make([]string, 0)
	var current strings.Builder
	var quote rune
	escaped := false

	flush := func() {
		if current.Len() == 0 {
			return
		}

		tokens = append(tokens, current.String())
		current.Reset()
	}

	for _, r := range input {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			current.WriteRune(r)
		}
	}

	if escaped {
		return nil, fmt.Errorf("trailing escape")
	}

	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}

	flush()

	return tokens, nil
}
