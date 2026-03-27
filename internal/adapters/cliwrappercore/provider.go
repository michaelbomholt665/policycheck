// internal/adapters/cliwrappercore/provider.go
package cliwrappercore

import (
	"context"
	"fmt"

	core "policycheck/internal/cliwrapper"
	"policycheck/internal/ports"
)

// Provider adapts pure wrapper core functions into the CLIWrapperCore contract.
type Provider struct{}

// DefaultWrapperConfig returns the shared default wrapper config.
func (Provider) DefaultWrapperConfig() ports.WrapperConfig {
	return fromCoreWrapperConfig(core.DefaultWrapperConfig())
}

// LoadActiveConfig loads the merged wrapper config for startDir.
func (Provider) LoadActiveConfig(startDir string) (ports.WrapperLoadResult, error) {
	globalConfigPath, err := core.DefaultGlobalConfigPath()
	if err != nil {
		return ports.WrapperLoadResult{}, fmt.Errorf("resolve global wrapper config path: %w", err)
	}

	loader := core.WrapperConfigLoader{
		GlobalConfigPath: globalConfigPath,
		StartDir:         startDir,
	}
	result, err := loader.Load()
	if err != nil {
		return ports.WrapperLoadResult{}, fmt.Errorf("load wrapper config: %w", err)
	}

	return fromCoreLoadResult(result), nil
}

// Detect classifies wrapper CLI args.
func (Provider) Detect(args []string, macroNames []string) ports.WrapperMode {
	return fromCoreWrapperMode(core.WrapperDetector{}.Detect(args, macroNames))
}

// ParseInstallRequest parses package-manager install args.
func (Provider) ParseInstallRequest(args []string) (ports.InstallRequest, error) {
	request, err := core.ParseInstallRequest(args)
	if err != nil {
		return ports.InstallRequest{}, err
	}

	return fromCoreInstallRequest(request), nil
}

// SplitChain splits a tooling chain at the shared -then token.
func (Provider) SplitChain(args []string) ([]string, []string, bool) {
	return core.SplitChain(args)
}

// RunChain runs the gate command before the main command.
func (Provider) RunChain(ctx context.Context, gate []string, main []string, exec ports.ExecFunc) error {
	return core.RunChain(ctx, gate, main, core.ExecFunc(exec))
}

// ParseSeverity parses a severity label with shared wrapper semantics.
func (Provider) ParseSeverity(value string) (ports.WrapperSeverity, error) {
	severity, err := core.ParseSeverity(value)
	if err != nil {
		return ports.SeverityInfo, err
	}

	return fromCoreSeverity(severity), nil
}

// CanonicalSeverityLabel returns the canonical label for a severity value.
func (Provider) CanonicalSeverityLabel(severity ports.WrapperSeverity) string {
	return core.CanonicalSeverityLabel(toCoreSeverity(severity))
}

// EvaluateSecurityPolicy evaluates advisories against an explicit wrapper policy.
func (Provider) EvaluateSecurityPolicy(cfg ports.WrapperSecurityConfig, advisories []ports.WrapperAdvisory) ports.SecurityResult {
	return fromCoreSecurityResult(core.EvaluateSecurityPolicy(toCoreSecurityConfig(cfg), toCoreAdvisories(advisories)))
}

// IsRiskOverrideAllowed reports whether allowRisk covers blockedSeverity.
func (Provider) IsRiskOverrideAllowed(allowRisk string, blockedSeverity ports.WrapperSeverity) (bool, error) {
	return core.IsRiskOverrideAllowed(allowRisk, toCoreSeverity(blockedSeverity))
}

func fromCoreSeverity(severity core.Severity) ports.WrapperSeverity {
	return ports.WrapperSeverity(severity)
}

func toCoreSeverity(severity ports.WrapperSeverity) core.Severity {
	return core.Severity(severity)
}

func fromCoreSecurityConfig(cfg core.WrapperSecurityConfig) ports.WrapperSecurityConfig {
	return ports.WrapperSecurityConfig{
		BlockOn: append([]string(nil), cfg.BlockOn...),
		WarnOn:  append([]string(nil), cfg.WarnOn...),
		AllowOn: append([]string(nil), cfg.AllowOn...),
		OSVMode: cfg.OSVMode,
	}
}

func toCoreSecurityConfig(cfg ports.WrapperSecurityConfig) core.WrapperSecurityConfig {
	return core.WrapperSecurityConfig{
		BlockOn: append([]string(nil), cfg.BlockOn...),
		WarnOn:  append([]string(nil), cfg.WarnOn...),
		AllowOn: append([]string(nil), cfg.AllowOn...),
		OSVMode: cfg.OSVMode,
	}
}

func fromCoreMacroConfigs(macros []core.WrapperMacroConfig) []ports.WrapperMacroConfig {
	result := make([]ports.WrapperMacroConfig, len(macros))
	for index, macro := range macros {
		result[index] = ports.WrapperMacroConfig{
			Name:        macro.Name,
			Description: macro.Description,
			Steps:       append([]string(nil), macro.Steps...),
			OnFailure:   macro.OnFailure,
		}
	}

	return result
}

func fromCoreWrapperConfig(cfg core.WrapperConfig) ports.WrapperConfig {
	return ports.WrapperConfig{
		Security: fromCoreSecurityConfig(cfg.Security),
		Tooling: ports.WrapperToolingConfig{
			Gates: fromCoreToolingGates(cfg.Tooling.Gates),
		},
		Macros: fromCoreMacroConfigs(cfg.Macros),
		UI: ports.WrapperUIConfig{
			Color:   cfg.UI.Color,
			Verbose: cfg.UI.Verbose,
		},
	}
}

func fromCoreToolingGates(gates []core.WrapperToolingGate) []ports.WrapperToolingGate {
	result := make([]ports.WrapperToolingGate, len(gates))
	for index, gate := range gates {
		result[index] = ports.WrapperToolingGate{
			Name: gate.Name,
			Gate: gate.Gate,
			Run:  gate.Run,
		}
	}

	return result
}

func fromCoreLoadResult(result core.WrapperLoadResult) ports.WrapperLoadResult {
	return ports.WrapperLoadResult{
		Merged:     fromCoreWrapperConfig(result.Merged),
		GlobalPath: result.GlobalPath,
		RepoPath:   result.RepoPath,
	}
}

func fromCoreWrapperMode(mode core.WrapperMode) ports.WrapperMode {
	return ports.WrapperMode(mode)
}

func fromCoreInstallRequest(request core.InstallRequest) ports.InstallRequest {
	return ports.InstallRequest{
		RawArgs:      append([]string(nil), request.RawArgs...),
		Manager:      request.Manager,
		Ecosystem:    ports.WrapperEcosystem(request.Ecosystem),
		Action:       ports.PackageManagerAction(request.Action),
		Packages:     append([]string(nil), request.Packages...),
		LockfileHint: request.LockfileHint,
	}
}

func toCoreAdvisories(advisories []ports.WrapperAdvisory) []core.Advisory {
	result := make([]core.Advisory, len(advisories))
	for index, advisory := range advisories {
		result[index] = core.Advisory{
			ID:       advisory.ID,
			Summary:  advisory.Summary,
			Severity: advisory.Severity,
		}
	}

	return result
}

func fromCoreSecurityResult(result core.SecurityResult) ports.SecurityResult {
	return ports.SecurityResult{
		Decision:         ports.SecurityDecision(result.Decision),
		Advisories:       fromCoreAdvisories(result.Advisories),
		BlockReason:      result.BlockReason,
		BlockingSeverity: ports.WrapperSeverity(result.BlockingSeverity),
	}
}

func fromCoreAdvisories(advisories []core.Advisory) []ports.WrapperAdvisory {
	result := make([]ports.WrapperAdvisory, len(advisories))
	for index, advisory := range advisories {
		result[index] = ports.WrapperAdvisory{
			ID:       advisory.ID,
			Summary:  advisory.Summary,
			Severity: advisory.Severity,
		}
	}

	return result
}
