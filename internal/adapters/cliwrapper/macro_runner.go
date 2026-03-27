// internal/adapters/cliwrapper/macro_runner.go
package cliwrapper

import (
	"context"
	"fmt"
	"os"

	core "policycheck/internal/cliwrapper"
	"policycheck/internal/ports"
)

// MacroRunnerAdapter loads wrapper config and executes the named macro.
type MacroRunnerAdapter struct {
	loadConfig func() (ports.WrapperConfig, error)
	exec       ports.ExecFunc
	vars       map[string]string
}

// NewMacroRunnerAdapter returns a macro runner with production defaults.
func NewMacroRunnerAdapter() *MacroRunnerAdapter {
	return NewMacroRunnerAdapterWithDeps(loadActiveAdapterConfig, defaultExecFunc, nil)
}

// NewMacroRunnerAdapterWithDeps returns a macro runner with injected seams for tests.
func NewMacroRunnerAdapterWithDeps(
	loadConfig func() (ports.WrapperConfig, error),
	exec ports.ExecFunc,
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

	runner := core.MacroRunner{
		Macros: toCoreWrapperMacros(cfg.Macros),
		Exec:   core.MacroExecFunc(a.exec),
		Vars:   a.vars,
	}

	if err := runner.RunMacro(ctx, name); err != nil {
		return fmt.Errorf("macro runner adapter: %w", err)
	}

	return nil
}

func loadActiveAdapterConfig() (ports.WrapperConfig, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return ports.WrapperConfig{}, fmt.Errorf("get working directory: %w", err)
	}

	coreProvider, err := resolveWrapperCore()
	if err != nil {
		return ports.WrapperConfig{}, fmt.Errorf("resolve wrapper core: %w", err)
	}

	result, err := coreProvider.LoadActiveConfig(workingDir)
	if err != nil {
		return ports.WrapperConfig{}, fmt.Errorf("load wrapper config: %w", err)
	}

	return result.Merged, nil
}

func toCoreWrapperMacros(macros []ports.WrapperMacroConfig) []core.WrapperMacroConfig {
	result := make([]core.WrapperMacroConfig, len(macros))
	for index, macro := range macros {
		result[index] = core.WrapperMacroConfig{
			Name:        macro.Name,
			Description: macro.Description,
			Steps:       append([]string(nil), macro.Steps...),
			OnFailure:   macro.OnFailure,
		}
	}

	return result
}
