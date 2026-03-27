// internal/ports/cliwrapper_core_port.go
// Declares the router port for shared CLI wrapper core logic.
// Keeps wrapper adapters bound to a contract instead of importing core code directly.
package ports

import "context"

// WrapperSeverity represents a wrapper security severity level ordered from least to most severe.
type WrapperSeverity int

const (
	// SeverityInfo is the informational severity level.
	SeverityInfo WrapperSeverity = iota
	// SeverityLow is a low-risk advisory.
	SeverityLow
	// SeverityModerate is a moderate-risk advisory.
	SeverityModerate
	// SeverityHigh is a high-risk advisory.
	SeverityHigh
	// SeverityCritical is a critical-risk advisory.
	SeverityCritical
)

// WrapperSecurityConfig holds the security-gate policy for the wrapper subsystem.
type WrapperSecurityConfig struct {
	BlockOn []string `toml:"block_on"`
	WarnOn  []string `toml:"warn_on"`
	AllowOn []string `toml:"allow_on"`
	OSVMode string   `toml:"osv_mode"`
}

// WrapperToolingGate controls a named tooling gate pair.
type WrapperToolingGate struct {
	Name string `toml:"-"`
	Gate string `toml:"gate"`
	Run  string `toml:"run"`
}

// WrapperToolingConfig groups named tooling-gate entries.
type WrapperToolingConfig struct {
	Gates []WrapperToolingGate `toml:"-"`
}

// WrapperMacroConfig describes a registered wrapper macro.
type WrapperMacroConfig struct {
	Name        string   `toml:"-"`
	Description string   `toml:"description"`
	Steps       []string `toml:"steps"`
	OnFailure   string   `toml:"on_failure"`
}

// WrapperUIConfig controls output rendering for the wrapper subsystem.
type WrapperUIConfig struct {
	Color   *bool `toml:"color"`
	Verbose *bool `toml:"verbose"`
}

// WrapperConfig is the root config schema for the wrapper subsystem.
type WrapperConfig struct {
	Security WrapperSecurityConfig `toml:"security"`
	Tooling  WrapperToolingConfig  `toml:"tooling"`
	Macros   []WrapperMacroConfig  `toml:"-"`
	UI       WrapperUIConfig       `toml:"ui"`
}

// WrapperLoadResult contains the outcome of a successful wrapper config load.
type WrapperLoadResult struct {
	Merged     WrapperConfig
	GlobalPath string
	RepoPath   string
}

// RiskBlockError reports a block decision together with the matched severity.
type RiskBlockError struct {
	Severity WrapperSeverity
	Reason   string
}

// Error returns the block reason.
func (e *RiskBlockError) Error() string {
	return e.Reason
}

// WrapperMode classifies an incoming CLI args slice into a wrapper dispatch lane.
type WrapperMode int

const (
	// ModePassthrough forwards the command directly to the executor.
	ModePassthrough WrapperMode = iota
	// ModePackageGate routes through the package-security gate.
	ModePackageGate
	// ModeToolingGate routes through the tooling-gate chain.
	ModeToolingGate
	// ModeMacroRun routes through the macro runner.
	ModeMacroRun
	// ModeFormatHeaders routes through the header formatter.
	ModeFormatHeaders
)

// ThenToken separates the gate and main commands in a tooling chain.
const ThenToken = "-then"

// ExecFunc is the function signature used by wrapper core chain execution.
type ExecFunc func(ctx context.Context, args []string) error

// WrapperEcosystem identifies the package ecosystem associated with an install request.
type WrapperEcosystem string

const (
	// EcosystemNPM is the npm / Node.js ecosystem.
	EcosystemNPM WrapperEcosystem = "npm"
	// EcosystemPyPI is the Python Package Index ecosystem.
	EcosystemPyPI WrapperEcosystem = "PyPI"
	// EcosystemGo is the Go module ecosystem.
	EcosystemGo WrapperEcosystem = "Go"
	// EcosystemPyPIUV is the uv-managed Python ecosystem.
	EcosystemPyPIUV WrapperEcosystem = "PyPI"
)

// PackageManagerAction is the normalised action verb for an install request.
type PackageManagerAction string

const (
	// ActionInstall covers "install", "add", and "get".
	ActionInstall PackageManagerAction = "install"
)

// InstallRequest captures the structured result of parsing a package-manager install command.
type InstallRequest struct {
	RawArgs      []string
	Manager      string
	Ecosystem    WrapperEcosystem
	Action       PackageManagerAction
	Packages     []string
	LockfileHint string
}

// SecurityDecision is the outcome of a security gate evaluation.
type SecurityDecision int

const (
	// DecisionAllow allows the wrapped command to continue.
	DecisionAllow SecurityDecision = iota
	// DecisionBlock blocks the wrapped command.
	DecisionBlock
	// DecisionScannerFailure blocks because scanning failed.
	DecisionScannerFailure
)

// WrapperAdvisory is a single vulnerability record returned from the security backend.
type WrapperAdvisory struct {
	ID       string
	Summary  string
	Severity string
}

// SecurityResult is the structured outcome of a gate evaluation.
type SecurityResult struct {
	Decision         SecurityDecision
	Advisories       []WrapperAdvisory
	BlockReason      string
	BlockingSeverity WrapperSeverity
}

// CLIWrapperCore is the shared wrapper-core contract resolved through the router.
type CLIWrapperCore interface {
	DefaultWrapperConfig() WrapperConfig
	LoadActiveConfig(startDir string) (WrapperLoadResult, error)
	Detect(args []string, macroNames []string) WrapperMode
	ParseInstallRequest(args []string) (InstallRequest, error)
	SplitChain(args []string) (gate []string, main []string, chained bool)
	RunChain(ctx context.Context, gate []string, main []string, exec ExecFunc) error
	ParseSeverity(value string) (WrapperSeverity, error)
	CanonicalSeverityLabel(severity WrapperSeverity) string
	EvaluateSecurityPolicy(cfg WrapperSecurityConfig, advisories []WrapperAdvisory) SecurityResult
	IsRiskOverrideAllowed(allowRisk string, blockedSeverity WrapperSeverity) (bool, error)
}
