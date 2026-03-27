// internal/adapters/cliwrapper/core.go
package cliwrapper

import (
	"fmt"

	"policycheck/internal/ports"
	"policycheck/internal/router"
)

// ExecFunc executes one wrapper command.
type ExecFunc = ports.ExecFunc

// Severity aliases the shared wrapper severity model for adapter consumers.
type Severity = ports.WrapperSeverity

const (
	// SeverityInfo is the informational severity level.
	SeverityInfo = ports.SeverityInfo
	// SeverityLow is the low severity level.
	SeverityLow = ports.SeverityLow
	// SeverityModerate is the moderate severity level.
	SeverityModerate = ports.SeverityModerate
	// SeverityHigh is the high severity level.
	SeverityHigh = ports.SeverityHigh
	// SeverityCritical is the critical severity level.
	SeverityCritical = ports.SeverityCritical
)

type (
	// WrapperSecurityConfig aliases the shared wrapper security config.
	WrapperSecurityConfig = ports.WrapperSecurityConfig
	// WrapperToolingGate aliases the shared named tooling-gate config.
	WrapperToolingGate = ports.WrapperToolingGate
	// WrapperToolingConfig aliases the shared tooling config container.
	WrapperToolingConfig = ports.WrapperToolingConfig
	// WrapperMacroConfig aliases the shared macro config schema.
	WrapperMacroConfig = ports.WrapperMacroConfig
	// WrapperUIConfig aliases the shared wrapper UI config schema.
	WrapperUIConfig = ports.WrapperUIConfig
	// WrapperConfig aliases the shared wrapper config root.
	WrapperConfig = ports.WrapperConfig
	// WrapperLoadResult aliases the shared wrapper config load result.
	WrapperLoadResult = ports.WrapperLoadResult
	// RiskBlockError aliases the shared risk-blocking error type.
	RiskBlockError = ports.RiskBlockError
)

// WrapperMode aliases the shared wrapper command classification enum.
type WrapperMode = ports.WrapperMode

const (
	// ModePassthrough forwards the command directly to the executor.
	ModePassthrough = ports.ModePassthrough
	// ModePackageGate routes through the package-security gate.
	ModePackageGate = ports.ModePackageGate
	// ModeToolingGate routes through the tooling-gate chain.
	ModeToolingGate = ports.ModeToolingGate
	// ModeMacroRun routes through the macro runner.
	ModeMacroRun = ports.ModeMacroRun
	// ModeFormatHeaders routes through the header formatter.
	ModeFormatHeaders = ports.ModeFormatHeaders
)

// ThenToken separates the gate and main commands in a tooling chain.
const ThenToken = ports.ThenToken

// Ecosystem aliases the shared package ecosystem identifier.
type Ecosystem = ports.WrapperEcosystem

const (
	// EcosystemNPM is the npm ecosystem.
	EcosystemNPM = ports.EcosystemNPM
	// EcosystemPyPI is the Python package ecosystem.
	EcosystemPyPI = ports.EcosystemPyPI
	// EcosystemGo is the Go module ecosystem.
	EcosystemGo = ports.EcosystemGo
	// EcosystemPyPIUV is the uv-managed PyPI ecosystem.
	EcosystemPyPIUV = ports.EcosystemPyPIUV
)

// PackageManagerAction aliases the shared install-action enum.
type PackageManagerAction = ports.PackageManagerAction

// ActionInstall is the shared install action.
const ActionInstall = ports.ActionInstall

// InstallRequest aliases the shared parsed install request type.
type InstallRequest = ports.InstallRequest

// SecurityDecision aliases the shared security decision enum.
type SecurityDecision = ports.SecurityDecision

const (
	// DecisionAllow allows the wrapped command to continue.
	DecisionAllow = ports.DecisionAllow
	// DecisionBlock blocks the wrapped command.
	DecisionBlock = ports.DecisionBlock
	// DecisionScannerFailure blocks because scanning failed.
	DecisionScannerFailure = ports.DecisionScannerFailure
)

type (
	// Advisory aliases the shared vulnerability advisory type.
	Advisory = ports.WrapperAdvisory
	// SecurityResult aliases the shared security evaluation result type.
	SecurityResult = ports.SecurityResult
)

func resolveWrapperCore() (ports.CLIWrapperCore, error) {
	raw, err := router.RouterResolveProvider(router.PortCLIWrapperCore)
	if err != nil {
		return nil, fmt.Errorf("resolve CLIWrapperCore: %w", err)
	}

	core, ok := raw.(ports.CLIWrapperCore)
	if !ok {
		return nil, fmt.Errorf("provider does not implement CLIWrapperCore")
	}

	return core, nil
}
