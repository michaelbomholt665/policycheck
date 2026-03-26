package cliwrapper

import (
	"context"
	"errors"
	"fmt"
)

// ExecFunc executes one wrapper command.
type ExecFunc func(ctx context.Context, args []string) error

// Severity represents a blocking-threshold category for the wrapper security gate.
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

var severityNames = map[string]Severity{
	"low":      SeverityLow,
	"medium":   SeverityMedium,
	"high":     SeverityHigh,
	"critical": SeverityCritical,
}

// ParseSeverity converts a string label to a Severity value.
func ParseSeverity(value string) (Severity, error) {
	severity, ok := severityNames[value]
	if !ok {
		return SeverityLow, fmt.Errorf("cliwrapper adapter: unknown severity %q", value)
	}

	return severity, nil
}

// SeverityAtLeast reports whether candidate is at least as strict as base.
func SeverityAtLeast(candidate, base Severity) bool {
	return candidate >= base
}

// WrapperSecurityConfig holds the security-gate policy for the wrapper subsystem.
type WrapperSecurityConfig struct {
	BlockThreshold string
}

// WrapperMacroConfig describes a registered wrapper macro.
type WrapperMacroConfig struct {
	Name  string
	Steps []string
}

// WrapperConfig is the adapter-local wrapper config schema.
type WrapperConfig struct {
	Security WrapperSecurityConfig
	Macros   []WrapperMacroConfig
}

// WrapperMode classifies an incoming CLI args slice into a wrapper dispatch lane.
type WrapperMode int

const (
	// ModePassthrough means the args do not match any wrapper feature.
	ModePassthrough WrapperMode = iota
	// ModePackageGate means the args represent a package installation command.
	ModePackageGate
	// ModeToolingGate means the args invoke an external tool controlled by the wrapper.
	ModeToolingGate
	// ModeMacroRun means the args reference a registered wrapper macro.
	ModeMacroRun
	// ModeFormatHeaders means the args request wrapper header formatting.
	ModeFormatHeaders
)

var packageInstallSubcmds = map[string][]string{
	"go":  {"get"},
	"pip": {"install"},
	"npm": {"install", "i"},
	"uv":  {"add", "pip", "install"},
}

var toolingGateManagers = []string{"go", "pip", "npm", "uv", "node"}

var formatHeadersSubcmds = map[string][]string{
	"go":  {"fmt", "headers"},
	"pip": {"fmt", "headers"},
}

// WrapperDetector classifies CLI args into a WrapperMode.
type WrapperDetector struct{}

// Detect returns the WrapperMode that best matches args.
func (d WrapperDetector) Detect(args []string, macroNames []string) WrapperMode {
	if len(args) == 0 {
		return ModePassthrough
	}

	if isMacroRun(args[0], macroNames) {
		return ModeMacroRun
	}

	if isFormatHeaders(args) {
		return ModeFormatHeaders
	}

	if isPackageGate(args) {
		return ModePackageGate
	}

	if isToolingGate(args[0]) {
		return ModeToolingGate
	}

	return ModePassthrough
}

func isMacroRun(name string, macroNames []string) bool {
	for _, macroName := range macroNames {
		if macroName == name {
			return true
		}
	}

	return false
}

func isFormatHeaders(args []string) bool {
	if len(args) < 3 {
		return false
	}

	subcommands, ok := formatHeadersSubcmds[args[0]]
	if !ok {
		return false
	}

	return sliceContains(subcommands, args[1]) && sliceContains(subcommands, args[2])
}

func isPackageGate(args []string) bool {
	if len(args) < 2 {
		return false
	}

	subcommands, ok := packageInstallSubcmds[args[0]]
	if !ok {
		return false
	}

	return sliceContains(subcommands, args[1])
}

func isToolingGate(name string) bool {
	return sliceContains(toolingGateManagers, name)
}

func sliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
}

// ThenToken separates the gate command from the main command in a chain.
const ThenToken = "-then"

// SplitChain splits args at the first ThenToken.
func SplitChain(args []string) (gate, main []string, chained bool) {
	for index, token := range args {
		if token == ThenToken {
			return args[:index], args[index+1:], true
		}
	}

	return nil, args, false
}

// RunChain executes the gate command first and the main command only on success.
func RunChain(ctx context.Context, gate, main []string, exec ExecFunc) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("chain: context cancelled before gate: %w", err)
	}

	if err := exec(ctx, gate); err != nil {
		return fmt.Errorf("chain: gate command failed: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("chain: context cancelled before main: %w", err)
	}

	if err := exec(ctx, main); err != nil {
		return fmt.Errorf("chain: main command failed: %w", err)
	}

	return nil
}

// Ecosystem identifies the package ecosystem associated with an install request.
type Ecosystem string

const (
	// EcosystemNPM is the npm ecosystem.
	EcosystemNPM Ecosystem = "npm"
	// EcosystemPyPI is the Python Package Index ecosystem.
	EcosystemPyPI Ecosystem = "PyPI"
	// EcosystemGo is the Go module ecosystem.
	EcosystemGo Ecosystem = "Go"
	// EcosystemPyPIUV is the uv-managed Python ecosystem.
	EcosystemPyPIUV Ecosystem = "PyPI"
)

// PackageManagerAction is the normalised action verb for an install request.
type PackageManagerAction string

const (
	// ActionInstall covers package acquisition actions such as install or get.
	ActionInstall PackageManagerAction = "install"
)

// InstallRequest captures the structured result of parsing a package-manager install command.
type InstallRequest struct {
	RawArgs      []string
	Manager      string
	Ecosystem    Ecosystem
	Action       PackageManagerAction
	Packages     []string
	LockfileHint string
}

// ErrUnsupportedManager is returned when the args do not identify a supported install action.
var ErrUnsupportedManager = errors.New("unsupported package manager or subcommand")

type managerMeta struct {
	installSubcmds []string
	ecosystem      Ecosystem
	lockfileHint   string
}

var managerTable = map[string]managerMeta{
	"npm": {
		installSubcmds: []string{"install", "i"},
		ecosystem:      EcosystemNPM,
		lockfileHint:   "package-lock.json",
	},
	"pip": {
		installSubcmds: []string{"install"},
		ecosystem:      EcosystemPyPI,
		lockfileHint:   "requirements.txt",
	},
	"go": {
		installSubcmds: []string{"get"},
		ecosystem:      EcosystemGo,
		lockfileHint:   "go.sum",
	},
	"uv": {
		installSubcmds: []string{"add", "install"},
		ecosystem:      EcosystemPyPIUV,
		lockfileHint:   "uv.lock",
	},
}

// ParseInstallRequest converts raw CLI args into a structured InstallRequest.
func ParseInstallRequest(args []string) (InstallRequest, error) {
	if len(args) == 0 {
		return InstallRequest{}, fmt.Errorf("package parser: empty args: %w", ErrUnsupportedManager)
	}

	meta, ok := managerTable[args[0]]
	if !ok {
		return InstallRequest{}, fmt.Errorf(
			"package parser: unsupported manager %q: %w", args[0], ErrUnsupportedManager,
		)
	}

	if len(args) < 2 || !sliceContains(meta.installSubcmds, args[1]) {
		return InstallRequest{}, fmt.Errorf(
			"package parser: %q is not an install subcommand for %q: %w",
			subcmdOrEmpty(args), args[0], ErrUnsupportedManager,
		)
	}

	return InstallRequest{
		RawArgs:      args,
		Manager:      args[0],
		Ecosystem:    meta.ecosystem,
		Action:       ActionInstall,
		Packages:     extractPackages(args),
		LockfileHint: meta.lockfileHint,
	}, nil
}

func extractPackages(args []string) []string {
	if len(args) <= 2 {
		return nil
	}

	packages := make([]string, len(args)-2)
	copy(packages, args[2:])

	return packages
}

func subcmdOrEmpty(args []string) string {
	if len(args) >= 2 {
		return args[1]
	}

	return "<none>"
}

// SecurityDecision is the outcome of a security gate evaluation.
type SecurityDecision int

const (
	// DecisionAllow means the package check may proceed.
	DecisionAllow SecurityDecision = iota
	// DecisionBlock means at least one advisory exceeds the block threshold.
	DecisionBlock
	// DecisionScannerFailure means the scan could not complete and is treated as a block.
	DecisionScannerFailure
)

// Advisory is a single vulnerability record returned from the security backend.
type Advisory struct {
	ID       string
	Summary  string
	Severity string
}

// SecurityResult is the structured outcome of a gate evaluation.
type SecurityResult struct {
	Decision    SecurityDecision
	Advisories  []Advisory
	BlockReason string
}

// EvaluateSeverity applies threshold to advisories and returns a SecurityResult.
func EvaluateSeverity(threshold Severity, advisories []Advisory) SecurityResult {
	for _, advisory := range advisories {
		advisorySeverity, err := ParseSeverity(advisory.Severity)
		if err != nil {
			advisorySeverity = SeverityCritical
		}

		if SeverityAtLeast(advisorySeverity, threshold) {
			return SecurityResult{
				Decision:    DecisionBlock,
				Advisories:  advisories,
				BlockReason: fmt.Sprintf("advisory %s (%s) meets or exceeds block threshold", advisory.ID, advisory.Severity),
			}
		}
	}

	return SecurityResult{
		Decision:   DecisionAllow,
		Advisories: advisories,
	}
}
