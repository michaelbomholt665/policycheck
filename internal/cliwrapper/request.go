// internal/cliwrapper/request.go
// Defines wrapper request and ecosystem value types shared across the subsystem.
// Keeps common wrapper domain models free of adapter dependencies and side effects.

// Package cliwrapper provides the core domain types and pure functions for the
// CLI wrapper subsystem.
//
// Package Concerns:
//   - Value types used across the wrapper subsystem (InstallRequest, etc.)
//   - Pure parsers and decision functions with no side-effects
//   - Zero adapter imports — this package knows only about its own types
package cliwrapper

// Ecosystem identifies the package ecosystem associated with an install request.
type Ecosystem string

const (
	// EcosystemNPM is the npm / Node.js ecosystem.
	EcosystemNPM Ecosystem = "npm"
	// EcosystemPyPI is the Python Package Index ecosystem.
	EcosystemPyPI Ecosystem = "PyPI"
	// EcosystemGo is the Go module ecosystem.
	EcosystemGo Ecosystem = "Go"
	// EcosystemPyPIUV is the uv-managed Python ecosystem (same backend as PyPI).
	EcosystemPyPIUV Ecosystem = "PyPI"
)

// PackageManagerAction is the normalised action verb for an install request.
type PackageManagerAction string

const (
	// ActionInstall covers "install", "add", "get" — package acquisition.
	ActionInstall PackageManagerAction = "install"
)

// InstallRequest captures the structured result of parsing a package-manager
// install command from raw CLI args.
//
// InstallRequest is serializable for log output and test assertions.
type InstallRequest struct {
	// RawArgs is the original args slice preserved for error reporting.
	RawArgs []string
	// Manager is the package manager name (e.g. "npm", "pip").
	Manager string
	// Ecosystem is the vulnerability-database ecosystem name for PURL construction.
	Ecosystem Ecosystem
	// Action is the normalised install action.
	Action PackageManagerAction
	// Packages is the ordered list of package specifiers from the args.
	// Each entry may include a version constraint (e.g. "requests==2.31.0").
	Packages []string
	// LockfileHint is the expected lockfile name for this manager.
	LockfileHint string
}
