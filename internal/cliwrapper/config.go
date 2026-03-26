// internal/cliwrapper/config.go
package cliwrapper

import (
	"errors"
	"fmt"
)

// Severity represents a blocking-threshold category for the wrapper security gate.
// Higher numeric values indicate greater severity.
type Severity int

const (
	// SeverityLow is the lowest severity level.
	SeverityLow Severity = iota
	// SeverityMedium is the medium severity level.
	SeverityMedium
	// SeverityHigh is the high severity level.
	SeverityHigh
	// SeverityCritical is the maximum severity level.
	SeverityCritical
)

// severityNames maps canonical string labels to Severity values.
var severityNames = map[string]Severity{
	"low":      SeverityLow,
	"medium":   SeverityMedium,
	"high":     SeverityHigh,
	"critical": SeverityCritical,
}

// ParseSeverity converts a string label to a Severity value.
// Returns an error if the label is not recognised.
func ParseSeverity(s string) (Severity, error) {
	if v, ok := severityNames[s]; ok {
		return v, nil
	}

	return SeverityLow, fmt.Errorf("cliwrapper: unknown severity %q", s)
}

// SeverityAtLeast reports whether candidate is at least as strict as base.
// Returns true if candidate >= base.
func SeverityAtLeast(candidate, base Severity) bool {
	return candidate >= base
}

// WrapperSecurityConfig holds the security-gate policy for the wrapper subsystem.
type WrapperSecurityConfig struct {
	// BlockThreshold is the minimum severity that causes the gate to block.
	// Recognised values: "low", "medium", "high", "critical".
	BlockThreshold string `toml:"block_threshold"`
}

// WrapperToolingGate controls whether a specific external tool can be invoked.
type WrapperToolingGate struct {
	// Name is the CLI tool name (e.g. "go", "pip", "npm").
	Name string `toml:"name"`
	// Enabled controls whether the tool is allowed through the gate.
	Enabled bool `toml:"enabled"`
}

// WrapperToolingConfig groups the tooling-gate policy entries.
type WrapperToolingConfig struct {
	// Gates is the ordered list of per-tool gate rules.
	Gates []WrapperToolingGate `toml:"gates"`
}

// WrapperMacroConfig describes a registered wrapper macro.
type WrapperMacroConfig struct {
	// Name is the macro identifier used in CLI args.
	Name string `toml:"name"`
	// Steps is the ordered list of shell commands the macro expands to.
	Steps []string `toml:"steps"`
	// OnFailure controls whether the macro halts or continues after a failed step.
	OnFailure string `toml:"on_failure"`
}

// WrapperUIConfig controls output rendering for the wrapper subsystem.
type WrapperUIConfig struct {
	// Color enables ANSI colour output.
	Color bool `toml:"color"`
	// Quiet suppresses informational messages; only errors are shown.
	Quiet bool `toml:"quiet"`
}

// WrapperConfig is the root config schema for the wrapper subsystem.
// Instances are loaded by WrapperConfigLoader and are intentionally separate
// from the policycheck analysis-engine config types.
type WrapperConfig struct {
	// Security holds the security-gate threshold for the wrapper subsystem.
	Security WrapperSecurityConfig `toml:"security"`
	// Tooling holds the per-tool gate configuration.
	Tooling WrapperToolingConfig `toml:"tooling"`
	// Macros is the registered macro list for this scope.
	Macros []WrapperMacroConfig `toml:"macros"`
	// UI controls the output rendering settings.
	UI WrapperUIConfig `toml:"ui"`
}

// ValidateWrapperConfig verifies structural correctness of a WrapperConfig.
// It does not enforce cross-config merge ordering rules; use
// ValidateConfigStrictnessOrder for that.
func ValidateWrapperConfig(cfg WrapperConfig) error {
	if cfg.Security.BlockThreshold != "" {
		if _, err := ParseSeverity(cfg.Security.BlockThreshold); err != nil {
			return fmt.Errorf("validate wrapper config: security: %w", err)
		}
	}

	for i, m := range cfg.Macros {
		if err := validateMacroShape(i, m); err != nil {
			return err
		}
	}

	return nil
}

// validateMacroShape checks that a macro entry has a non-empty name and at
// least one step.
func validateMacroShape(idx int, m WrapperMacroConfig) error {
	if m.Name == "" {
		return fmt.Errorf("validate wrapper config: macros[%d]: name must not be empty", idx)
	}

	if len(m.Steps) == 0 {
		return fmt.Errorf("validate wrapper config: macros[%d] %q: steps must not be empty", idx, m.Name)
	}

	if _, err := NormalizeMacroOnFailure(m.OnFailure); err != nil {
		return fmt.Errorf("validate wrapper config: macros[%d] %q: %w", idx, m.Name, err)
	}

	return nil
}

// ValidateConfigStrictnessOrder verifies that the repo config does not relax
// the security threshold set by the global config.
// repo.Security.BlockThreshold must be >= global.Security.BlockThreshold.
func ValidateConfigStrictnessOrder(global, repo WrapperConfig) error {
	if global.Security.BlockThreshold == "" {
		return nil
	}

	if repo.Security.BlockThreshold == "" {
		return nil
	}

	globalSev, err := ParseSeverity(global.Security.BlockThreshold)
	if err != nil {
		return fmt.Errorf("validate config strictness: global: %w", err)
	}

	repoSev, err := ParseSeverity(repo.Security.BlockThreshold)
	if err != nil {
		return fmt.Errorf("validate config strictness: repo: %w", err)
	}

	if !SeverityAtLeast(repoSev, globalSev) {
		return errors.New(
			"validate config strictness: repo block_threshold must not relax global threshold",
		)
	}

	return nil
}
