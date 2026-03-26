// internal/cliwrapper/config.go
// Defines wrapper configuration types and validation helpers for CLI policy.
// Keeps wrapper config parsing separate from command execution code paths.
package cliwrapper

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Severity represents a wrapper security severity level ordered from least to most severe.
type Severity int

const (
	// SeverityInfo is the informational severity level.
	SeverityInfo Severity = iota
	// SeverityLow is a low-risk advisory.
	SeverityLow
	// SeverityModerate is a moderate-risk advisory.
	SeverityModerate
	// SeverityHigh is a high-risk advisory.
	SeverityHigh
	// SeverityCritical is a critical-risk advisory.
	SeverityCritical
)

// severityNames maps accepted config labels to Severity values.
var severityNames = map[string]Severity{
	"info":     SeverityInfo,
	"low":      SeverityLow,
	"medium":   SeverityModerate,
	"moderate": SeverityModerate,
	"high":     SeverityHigh,
	"critical": SeverityCritical,
}

// ParseSeverity converts a config or advisory label to a Severity value.
func ParseSeverity(value string) (Severity, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if severity, ok := severityNames[normalized]; ok {
		return severity, nil
	}

	return SeverityInfo, fmt.Errorf("cliwrapper: unknown severity %q", value)
}

// SeverityAtLeast reports whether candidate is at least as strict as base.
func SeverityAtLeast(candidate, base Severity) bool {
	return candidate >= base
}

// CanonicalSeverityLabel returns the canonical uppercase representation of severity.
func CanonicalSeverityLabel(severity Severity) string {
	switch severity {
	case SeverityInfo:
		return "INFO"
	case SeverityLow:
		return "LOW"
	case SeverityModerate:
		return "MODERATE"
	case SeverityHigh:
		return "HIGH"
	case SeverityCritical:
		return "CRITICAL"
	default:
		return "CRITICAL"
	}
}

// WrapperSecurityConfig holds the security-gate policy for the wrapper subsystem.
type WrapperSecurityConfig struct {
	// BlockOn lists the severities that hard-block execution.
	BlockOn []string `toml:"block_on"`
	// WarnOn lists the severities that warn but do not hard-block.
	WarnOn []string `toml:"warn_on"`
	// AllowOn lists the severities that always proceed.
	AllowOn []string `toml:"allow_on"`
	// OSVMode chooses the OSV lookup backend.
	OSVMode string `toml:"osv_mode"`
}

// WrapperToolingGate controls a named tooling gate pair.
type WrapperToolingGate struct {
	// Name is the identifier used to refer to the tooling gate.
	Name string `toml:"-"`
	// Gate is the prerequisite command that must succeed first.
	Gate string `toml:"gate"`
	// Run is the main command that executes after Gate succeeds.
	Run string `toml:"run"`
}

// WrapperToolingConfig groups named tooling-gate entries.
type WrapperToolingConfig struct {
	// Gates is the ordered list of named tooling gates.
	Gates []WrapperToolingGate `toml:"-"`
}

// WrapperMacroConfig describes a registered wrapper macro.
type WrapperMacroConfig struct {
	// Name is the macro identifier used in CLI args.
	Name string `toml:"-"`
	// Description is the human-readable summary for the macro.
	Description string `toml:"description"`
	// Steps is the ordered list of shell commands the macro expands to.
	Steps []string `toml:"steps"`
	// OnFailure controls whether the macro halts or continues after a failed step.
	OnFailure string `toml:"on_failure"`
}

// WrapperUIConfig controls output rendering for the wrapper subsystem.
type WrapperUIConfig struct {
	// Color enables ANSI colour output.
	Color *bool `toml:"color"`
	// Verbose enables verbose wrapper logs.
	Verbose *bool `toml:"verbose"`
}

// WrapperConfig is the root config schema for the wrapper subsystem.
type WrapperConfig struct {
	// Security holds the security-gate policy.
	Security WrapperSecurityConfig `toml:"security"`
	// Tooling holds the named tooling-gate definitions.
	Tooling WrapperToolingConfig `toml:"tooling"`
	// Macros is the registered macro list for this scope.
	Macros []WrapperMacroConfig `toml:"-"`
	// UI controls wrapper rendering preferences.
	UI WrapperUIConfig `toml:"ui"`
}

// RiskBlockError reports a block decision together with the matched severity.
type RiskBlockError struct {
	Severity Severity
	Reason   string
}

// Error returns the block reason.
func (e *RiskBlockError) Error() string {
	return e.Reason
}

// DefaultWrapperConfig returns the effective wrapper defaults used when config is absent.
func DefaultWrapperConfig() WrapperConfig {
	color := true
	verbose := false

	return WrapperConfig{
		Security: WrapperSecurityConfig{
			BlockOn: []string{"CRITICAL", "HIGH"},
			WarnOn:  []string{"MODERATE"},
			AllowOn: []string{"LOW", "INFO"},
			OSVMode: "cli",
		},
		UI: WrapperUIConfig{
			Color:   &color,
			Verbose: &verbose,
		},
	}
}

// NormalizeWrapperConfig returns a copy with canonical severity labels and stable ordering.
func NormalizeWrapperConfig(cfg WrapperConfig) (WrapperConfig, error) {
	normalized := cfg

	blockOn, err := normalizeSeverityList(cfg.Security.BlockOn)
	if err != nil {
		return WrapperConfig{}, fmt.Errorf("normalize wrapper config: security.block_on: %w", err)
	}
	warnOn, err := normalizeSeverityList(cfg.Security.WarnOn)
	if err != nil {
		return WrapperConfig{}, fmt.Errorf("normalize wrapper config: security.warn_on: %w", err)
	}
	allowOn, err := normalizeSeverityList(cfg.Security.AllowOn)
	if err != nil {
		return WrapperConfig{}, fmt.Errorf("normalize wrapper config: security.allow_on: %w", err)
	}
	normalized.Security.BlockOn = blockOn
	normalized.Security.WarnOn = warnOn
	normalized.Security.AllowOn = allowOn
	normalized.Security.OSVMode = normalizeOSVMode(cfg.Security.OSVMode)

	for index, macro := range normalized.Macros {
		normalized.Macros[index].OnFailure, err = NormalizeMacroOnFailure(macro.OnFailure)
		if err != nil {
			return WrapperConfig{}, fmt.Errorf("normalize wrapper config: macro %q: %w", macro.Name, err)
		}
	}

	sortMacros(normalized.Macros)
	sortToolingGates(normalized.Tooling.Gates)

	return normalized, nil
}

// ValidateWrapperConfig verifies structural correctness of a WrapperConfig.
func ValidateWrapperConfig(cfg WrapperConfig) error {
	if _, err := normalizeSeverityList(cfg.Security.BlockOn); err != nil {
		return fmt.Errorf("validate wrapper config: security.block_on: %w", err)
	}
	if _, err := normalizeSeverityList(cfg.Security.WarnOn); err != nil {
		return fmt.Errorf("validate wrapper config: security.warn_on: %w", err)
	}
	if _, err := normalizeSeverityList(cfg.Security.AllowOn); err != nil {
		return fmt.Errorf("validate wrapper config: security.allow_on: %w", err)
	}

	for i, gate := range cfg.Tooling.Gates {
		if gate.Name == "" {
			return fmt.Errorf("validate wrapper config: tooling.gates[%d]: name must not be empty", i)
		}
		if strings.TrimSpace(gate.Gate) == "" {
			return fmt.Errorf("validate wrapper config: tooling.gates[%d] %q: gate must not be empty", i, gate.Name)
		}
		if strings.TrimSpace(gate.Run) == "" {
			return fmt.Errorf("validate wrapper config: tooling.gates[%d] %q: run must not be empty", i, gate.Name)
		}
	}

	for i, macro := range cfg.Macros {
		if err := validateMacroShape(i, macro); err != nil {
			return err
		}
	}

	return nil
}

// ValidateConfigStrictnessOrder verifies that repo block_on cannot relax global block_on.
func ValidateConfigStrictnessOrder(global, repo WrapperConfig) error {
	globalLevels, err := normalizeSeverityList(global.Security.BlockOn)
	if err != nil {
		return fmt.Errorf("validate config strictness: global: %w", err)
	}

	repoLevels, err := normalizeSeverityList(repo.Security.BlockOn)
	if err != nil {
		return fmt.Errorf("validate config strictness: repo: %w", err)
	}

	if len(globalLevels) == 0 || len(repoLevels) == 0 {
		return nil
	}

	for _, level := range globalLevels {
		if !slices.Contains(repoLevels, level) {
			return errors.New("validate config strictness: repo block_on must not relax global block_on")
		}
	}

	return nil
}

// IsRiskOverrideAllowed reports whether allowRisk is sufficient to override blockedSeverity.
func IsRiskOverrideAllowed(allowRisk string, blockedSeverity Severity) (bool, error) {
	if strings.TrimSpace(allowRisk) == "" {
		return false, nil
	}

	overrideSeverity, err := ParseSeverity(allowRisk)
	if err != nil {
		return false, fmt.Errorf("parse allow-risk: %w", err)
	}

	return overrideSeverity >= blockedSeverity, nil
}

// sortMacros sorts macros by name for deterministic merges and rendering.
func sortMacros(macros []WrapperMacroConfig) {
	slices.SortStableFunc(macros, func(left, right WrapperMacroConfig) int {
		return strings.Compare(left.Name, right.Name)
	})
}

// sortToolingGates sorts tooling gates by name for deterministic merges and rendering.
func sortToolingGates(gates []WrapperToolingGate) {
	slices.SortStableFunc(gates, func(left, right WrapperToolingGate) int {
		return strings.Compare(left.Name, right.Name)
	})
}

// normalizeSeverityList normalizes a severity label list to canonical uppercase values.
func normalizeSeverityList(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		severity, err := ParseSeverity(value)
		if err != nil {
			return nil, err
		}

		label := CanonicalSeverityLabel(severity)
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		normalized = append(normalized, label)
	}

	slices.SortStableFunc(normalized, func(left, right string) int {
		leftSeverity, _ := ParseSeverity(left)
		rightSeverity, _ := ParseSeverity(right)
		switch {
		case leftSeverity < rightSeverity:
			return 1
		case leftSeverity > rightSeverity:
			return -1
		default:
			return strings.Compare(left, right)
		}
	})

	return normalized, nil
}

// normalizeOSVMode returns a canonical OSV mode label.
func normalizeOSVMode(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}

	return strings.ToLower(strings.TrimSpace(value))
}

// validateMacroShape checks that a macro entry has a non-empty name and at least one step.
func validateMacroShape(idx int, macro WrapperMacroConfig) error {
	if macro.Name == "" {
		return fmt.Errorf("validate wrapper config: macros[%d]: name must not be empty", idx)
	}

	if len(macro.Steps) == 0 {
		return fmt.Errorf("validate wrapper config: macros[%d] %q: steps must not be empty", idx, macro.Name)
	}

	if _, err := NormalizeMacroOnFailure(macro.OnFailure); err != nil {
		return fmt.Errorf("validate wrapper config: macros[%d] %q: %w", idx, macro.Name, err)
	}

	return nil
}
