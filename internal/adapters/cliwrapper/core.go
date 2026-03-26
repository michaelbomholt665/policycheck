// internal/adapters/cliwrapper/core.go
package cliwrapper

import (
	"context"

	corecliwrapper "policycheck/internal/cliwrapper"
)

// ExecFunc executes one wrapper command.
type ExecFunc func(ctx context.Context, args []string) error

// Severity aliases the shared wrapper severity model for adapter consumers.
type Severity = corecliwrapper.Severity

const (
	// SeverityInfo is the informational severity level.
	SeverityInfo = corecliwrapper.SeverityInfo
	// SeverityLow is the low severity level.
	SeverityLow = corecliwrapper.SeverityLow
	// SeverityModerate is the moderate severity level.
	SeverityModerate = corecliwrapper.SeverityModerate
	// SeverityHigh is the high severity level.
	SeverityHigh = corecliwrapper.SeverityHigh
	// SeverityCritical is the critical severity level.
	SeverityCritical = corecliwrapper.SeverityCritical
)

type (
	// WrapperSecurityConfig aliases the shared wrapper security config.
	WrapperSecurityConfig = corecliwrapper.WrapperSecurityConfig
	// WrapperToolingGate aliases the shared named tooling-gate config.
	WrapperToolingGate = corecliwrapper.WrapperToolingGate
	// WrapperToolingConfig aliases the shared tooling config container.
	WrapperToolingConfig = corecliwrapper.WrapperToolingConfig
	// WrapperMacroConfig aliases the shared macro config schema.
	WrapperMacroConfig = corecliwrapper.WrapperMacroConfig
	// WrapperUIConfig aliases the shared wrapper UI config schema.
	WrapperUIConfig = corecliwrapper.WrapperUIConfig
	// WrapperConfig aliases the shared wrapper config root.
	WrapperConfig = corecliwrapper.WrapperConfig
	// WrapperConfigLoader aliases the shared wrapper config loader.
	WrapperConfigLoader = corecliwrapper.WrapperConfigLoader
	// RiskBlockError aliases the shared risk-blocking error type.
	RiskBlockError = corecliwrapper.RiskBlockError
)

// WrapperMode aliases the shared wrapper command classification enum.
type WrapperMode = corecliwrapper.WrapperMode

const (
	// ModePassthrough forwards the command directly to the executor.
	ModePassthrough = corecliwrapper.ModePassthrough
	// ModePackageGate routes through the package-security gate.
	ModePackageGate = corecliwrapper.ModePackageGate
	// ModeToolingGate routes through the tooling-gate chain.
	ModeToolingGate = corecliwrapper.ModeToolingGate
	// ModeMacroRun routes through the macro runner.
	ModeMacroRun = corecliwrapper.ModeMacroRun
	// ModeFormatHeaders routes through the header formatter.
	ModeFormatHeaders = corecliwrapper.ModeFormatHeaders
)

// WrapperDetector aliases the shared wrapper detector.
type WrapperDetector = corecliwrapper.WrapperDetector

// ThenToken separates the gate and main commands in a tooling chain.
const ThenToken = corecliwrapper.ThenToken

// Ecosystem aliases the shared package ecosystem identifier.
type Ecosystem = corecliwrapper.Ecosystem

const (
	// EcosystemNPM is the npm ecosystem.
	EcosystemNPM = corecliwrapper.EcosystemNPM
	// EcosystemPyPI is the Python package ecosystem.
	EcosystemPyPI = corecliwrapper.EcosystemPyPI
	// EcosystemGo is the Go module ecosystem.
	EcosystemGo = corecliwrapper.EcosystemGo
	// EcosystemPyPIUV is the uv-managed PyPI ecosystem.
	EcosystemPyPIUV = corecliwrapper.EcosystemPyPIUV
)

// PackageManagerAction aliases the shared install-action enum.
type PackageManagerAction = corecliwrapper.PackageManagerAction

// ActionInstall is the shared install action.
const ActionInstall = corecliwrapper.ActionInstall

// InstallRequest aliases the shared parsed install request type.
type InstallRequest = corecliwrapper.InstallRequest

// SecurityDecision aliases the shared security decision enum.
type SecurityDecision = corecliwrapper.SecurityDecision

const (
	// DecisionAllow allows the wrapped command to continue.
	DecisionAllow = corecliwrapper.DecisionAllow
	// DecisionBlock blocks the wrapped command.
	DecisionBlock = corecliwrapper.DecisionBlock
	// DecisionScannerFailure blocks because scanning failed.
	DecisionScannerFailure = corecliwrapper.DecisionScannerFailure
)

type (
	// Advisory aliases the shared vulnerability advisory type.
	Advisory = corecliwrapper.Advisory
	// SecurityResult aliases the shared security evaluation result type.
	SecurityResult = corecliwrapper.SecurityResult
)

// ErrUnsupportedManager aliases the shared unsupported-manager error sentinel.
var ErrUnsupportedManager = corecliwrapper.ErrUnsupportedManager

// ParseSeverity parses a severity label with shared wrapper semantics.
func ParseSeverity(value string) (Severity, error) {
	return corecliwrapper.ParseSeverity(value)
}

// SeverityAtLeast reports whether candidate is at least as strict as base.
func SeverityAtLeast(candidate, base Severity) bool {
	return corecliwrapper.SeverityAtLeast(candidate, base)
}

// CanonicalSeverityLabel returns the canonical label for a severity value.
func CanonicalSeverityLabel(severity Severity) string {
	return corecliwrapper.CanonicalSeverityLabel(severity)
}

// DefaultWrapperConfig returns the shared default wrapper config.
func DefaultWrapperConfig() WrapperConfig {
	return corecliwrapper.DefaultWrapperConfig()
}

// DefaultGlobalConfigPath returns the shared machine-global config path.
func DefaultGlobalConfigPath() (string, error) {
	return corecliwrapper.DefaultGlobalConfigPath()
}

// ParseInstallRequest parses package-manager args using the shared parser.
func ParseInstallRequest(args []string) (InstallRequest, error) {
	return corecliwrapper.ParseInstallRequest(args)
}

// SplitChain splits a tooling chain at the shared -then token.
func SplitChain(args []string) (gate, main []string, chained bool) {
	return corecliwrapper.SplitChain(args)
}

// RunChain runs the gate command before the main command.
func RunChain(ctx context.Context, gate, main []string, exec ExecFunc) error {
	return corecliwrapper.RunChain(ctx, gate, main, corecliwrapper.ExecFunc(exec))
}

// EvaluateSeverity evaluates advisories against a threshold severity.
func EvaluateSeverity(threshold Severity, advisories []Advisory) SecurityResult {
	return corecliwrapper.EvaluateSeverity(threshold, advisories)
}

// EvaluateSecurityPolicy evaluates advisories against an explicit wrapper policy.
func EvaluateSecurityPolicy(cfg WrapperSecurityConfig, advisories []Advisory) SecurityResult {
	return corecliwrapper.EvaluateSecurityPolicy(cfg, advisories)
}

// IsRiskOverrideAllowed reports whether allowRisk covers blockedSeverity.
func IsRiskOverrideAllowed(allowRisk string, blockedSeverity Severity) (bool, error) {
	return corecliwrapper.IsRiskOverrideAllowed(allowRisk, blockedSeverity)
}
