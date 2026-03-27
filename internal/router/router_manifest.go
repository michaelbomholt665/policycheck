// internal/router/router_manifest.go
// Contains the statically generated ordered declarations for base ports
// and optional extensions, acting as the ground-truth for boot sequences.

package router

// PortManifestEntry declares one generated router port constant.
type PortManifestEntry struct {
	Name  string
	Value string
}

// OptionalExtensionManifestEntry declares one generated optional router extension.
type OptionalExtensionManifestEntry struct {
	Name string
}

// DeclaredPorts defines the ordered router port declarations for generated router files.
var DeclaredPorts = []PortManifestEntry{
	{Name: "PortPrimary", Value: "primary"},
	{Name: "PortSecondary", Value: "secondary"},
	{Name: "PortTertiary", Value: "tertiary"},
	{Name: "PortOptional", Value: "optional"},
	{Name: "PortCLIStyle", Value: "cli-style"},
	{Name: "PortCLIChrome", Value: "cli-chrome"},
	{Name: "PortCLIInteraction", Value: "cli-interaction"}, {Name:

	// DeclaredOptionalExtensions defines the ordered optional router extension declarations.
	"PortConfig", Value: "config"}, {Name: "PortWalk", Value: "walk"}, {Name: "PortScanner", Value: "scanner"}, {Name: "PortReadFile", Value: "readfile"}, {Name: "PortCLIWrapperCore", Value: "cli-wrapper-core"}, {Name: "PortCLIWrapperDispatcher", Value: "cli-wrapper-dispatcher"}, {Name: "PortCLIWrapperSecurityGate", Value: "cli-wrapper-security"}, {Name: "PortCLIWrapperMacroRunner", Value: "cli-wrapper-macro"}, {Name: "PortCLIWrapperFormatter", Value: "cli-wrapper-format"},
}

var DeclaredOptionalExtensions = []OptionalExtensionManifestEntry{
	{Name: "prettystyle"},
	{Name: "charmcli"},
}
