// internal/cliwrapper/detector.go
// Classifies wrapper CLI arguments into gate, macro, format, or passthrough modes.
// Keeps wrapper dispatch selection as a pure decision layer with no I/O.
package cliwrapper

// WrapperMode classifies an incoming CLI args slice into a wrapper dispatch lane.
type WrapperMode int

const (
	// ModePassthrough means the args do not match any wrapper feature and
	// should be forwarded directly to the underlying tool.
	ModePassthrough WrapperMode = iota
	// ModePackageGate means the args represent a package-installation command
	// that must pass the security gate before continuing.
	ModePackageGate
	// ModeToolingGate means the args invoke an external tool that is
	// controlled by a wrapper tooling-gate rule.
	ModeToolingGate
	// ModeMacroRun means the args reference a registered wrapper macro.
	ModeMacroRun
	// ModeFormatHeaders means the args request wrapper header formatting (e.g. `go fmt headers`).
	ModeFormatHeaders
)

// packageInstallSubcmds lists the subcommand names that trigger the package gate
// for each package manager.
//
// The table is intentionally small while wrapper classification rules are still
// evolving; extend it as more managers are supported.
var packageInstallSubcmds = map[string][]string{
	"go":  {"get"},
	"pip": {"install"},
	"npm": {"install", "i"},
	"uv":  {"add", "pip", "install"},
}

// toolingGateManagers lists external tools whose top-level invocation triggers
// the tooling gate.
var toolingGateManagers = []string{"go", "pip", "npm", "uv", "node"}

// formatHeadersSubcmds maps manager name to subcommand tokens that trigger
// ModeFormatHeaders.
var formatHeadersSubcmds = map[string][]string{
	"go":  {"fmt", "headers"},
	"pip": {"fmt", "headers"},
}

// WrapperDetector classifies CLI args into a WrapperMode.
//
// The zero value is ready to use; no initialisation is required.
type WrapperDetector struct{}

// Detect returns the WrapperMode that best matches args.
//
// The classification precedence is:
//  1. MacroRun  — first arg matches a macro name from the registered list.
//  2. FormatHeaders — args match the format-headers subcommand pattern.
//  3. PackageGate — args look like a package-install invocation.
//  4. ToolingGate — first arg is a known managed tool.
//  5. Passthrough — none of the above.
//
// An empty or nil args slice is always ModePassthrough.
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

// isMacroRun checks whether name matches any registered macro.
func isMacroRun(name string, macroNames []string) bool {
	for _, m := range macroNames {
		if m == name {
			return true
		}
	}

	return false
}

// isFormatHeaders reports whether args look like a format-headers invocation.
func isFormatHeaders(args []string) bool {
	if len(args) < 3 {
		return false
	}

	subs, ok := formatHeadersSubcmds[args[0]]
	if !ok {
		return false
	}

	return sliceContains(subs, args[1]) && sliceContains(subs, args[2])
}

// isPackageGate reports whether args represent a recognised package-install
// invocation (manager + install subcommand).
func isPackageGate(args []string) bool {
	if len(args) < 2 {
		return false
	}

	subcmds, ok := packageInstallSubcmds[args[0]]
	if !ok {
		return false
	}

	return sliceContains(subcmds, args[1])
}

// isToolingGate reports whether name is a managed external tool.
func isToolingGate(name string) bool {
	return sliceContains(toolingGateManagers, name)
}

// sliceContains is a small linear-search helper for short string slices.
func sliceContains(slice []string, value string) bool {
	for _, s := range slice {
		if s == value {
			return true
		}
	}

	return false
}
