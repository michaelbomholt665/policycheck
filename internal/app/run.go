// internal/app/run.go
// Package app provides the shared binary dispatch seam between the wrapper
// and policycheck analysis surfaces.
package app

import (
	"context"
	"fmt"
	"os"

	"policycheck/internal/cliwrapper"
	"policycheck/internal/policycheck/cli"
)

// AnalysisHandler handles the policycheck analysis CLI dispatch.
type AnalysisHandler func(args []string) int

// WrapperHandler handles the CLI wrapper subsystem dispatch.
type WrapperHandler func(ctx context.Context, args []string) error

// ConfigHandler handles the shared wrapper config command surface.
type ConfigHandler func(args []string) error

// Run is the shared entrypoint for both wrapper and policycheck analysis.
func Run(ctx context.Context, args []string) int {
	return RunWithDependencies(ctx, args, cli.Run, func(ctx context.Context, args []string) error {
		wrapper, err := cliwrapper.WrapperBootEntry()
		if err != nil {
			return fmt.Errorf("wrapper boot: %w", err)
		}
		return wrapper.Dispatcher.Dispatch(ctx, args)
	}, RunConfigCommand)
}

// RunWithHandlers implements the shared binary dispatch logic between wrapper and analysis.
func RunWithHandlers(ctx context.Context, args []string, analysisHandler AnalysisHandler, wrapperHandler WrapperHandler) int {
	return RunWithDependencies(ctx, args, analysisHandler, wrapperHandler, RunConfigCommand)
}

// RunWithDependencies implements the shared binary dispatch logic between wrapper, config, and analysis.
func RunWithDependencies(
	ctx context.Context,
	args []string,
	analysisHandler AnalysisHandler,
	wrapperHandler WrapperHandler,
	configHandler ConfigHandler,
) int {
	if len(args) == 0 {
		return analysisHandler(args)
	}

	// Explicit analysis entry surface
	if args[0] == "check" {
		return analysisHandler(args[1:])
	}

	if args[0] == "config" {
		if err := configHandler(args[1:]); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "config failure: %v\n", err)
			return 1
		}
		return 0
	}

	// Load wrapper config to get macros for detection
	var macroNames []string
	workingDir, err := os.Getwd()
	if err == nil {
		globalPath, pathErr := cliwrapper.DefaultGlobalConfigPath()
		if pathErr == nil {
			loader := cliwrapper.WrapperConfigLoader{
				GlobalConfigPath: globalPath,
				StartDir:         workingDir,
			}
			if res, loadErr := loader.Load(); loadErr == nil {
				for _, m := range res.Merged.Macros {
					macroNames = append(macroNames, m.Name)
				}
			}
		}
	}

	// Classify the command
	detector := cliwrapper.WrapperDetector{}
	mode := detector.Detect(args, macroNames)

	if mode != cliwrapper.ModePassthrough {
		if err := wrapperHandler(ctx, args); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "wrapper failure: %v\n", err)
			return 1
		}
		return 0
	}

	// Fallback to analysis for everything else
	return analysisHandler(args)
}
