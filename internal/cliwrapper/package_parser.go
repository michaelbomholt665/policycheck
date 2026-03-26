// internal/cliwrapper/package_parser.go
package cliwrapper

import (
	"errors"
	"fmt"
)

// ErrUnsupportedManager is returned when the first args token is not a
// recognised package manager or the subcommand is not an install action.
var ErrUnsupportedManager = errors.New("unsupported package manager or subcommand")

// managerMeta holds the static metadata for a supported package manager.
type managerMeta struct {
	// installSubcmds is the set of subcommand tokens that trigger a package install.
	installSubcmds []string
	// ecosystem is the vulnerability-database ecosystem for PURL lookup.
	ecosystem Ecosystem
	// lockfileHint is the expected lockfile filename.
	lockfileHint string
}

// managerTable is the authoritative registry of supported package managers.
//
// Extend this table when adding support for a new manager; do not scatter
// manager-specific logic across the parser body.
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
//
// ParseInstallRequest is a pure function: no I/O, no side-effects, safe to
// call from tests without any setup.
//
// Returns ErrUnsupportedManager (wrapped) when:
//   - args is empty.
//   - args[0] is not a recognised package manager.
//   - args[1] is not an install subcommand for that manager.
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

	pkgs := extractPackages(args)

	return InstallRequest{
		RawArgs:      args,
		Manager:      args[0],
		Ecosystem:    meta.ecosystem,
		Action:       ActionInstall,
		Packages:     pkgs,
		LockfileHint: meta.lockfileHint,
	}, nil
}

// extractPackages returns the package specifiers that follow the subcommand.
// Returns nil (not an error) when no packages are listed after the subcommand.
func extractPackages(args []string) []string {
	if len(args) <= 2 {
		return nil
	}

	pkgs := make([]string, len(args)-2)
	copy(pkgs, args[2:])

	return pkgs
}

// subcmdOrEmpty returns args[1] for error reporting, or "<none>" when absent.
func subcmdOrEmpty(args []string) string {
	if len(args) >= 2 {
		return args[1]
	}

	return "<none>"
}
