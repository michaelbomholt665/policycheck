// Package ext provides the router extension wiring and boot orchestration.
//
// Package Concerns:
// - This package must never import internal packages that depend on router; ext handles all adapter wiring.
// - Callers boot via ext.RouterBootExtensions and resolve via RouterResolveProvider.
//
// # Optional Extensions
//
// Optional extensions are capability extensions that extend the base router without adding
// dependencies to the core router code. They load before application extensions during boot
// and are ideal for adding cross-cutting concerns like telemetry, logging, or metrics.
//
// To add a new optional extension:
//  1. Create the extension in internal/router/ext/extensions/<name>/
//  2. Reference it in the optionalExtensions slice below
//  3. Optional extensions that fail to load produce warnings but do not halt boot
//
// # Required Extensions
//
// Required extensions are core functionality that must be present for the router to operate.
// They are defined in extensions.go and boot failure is fatal if they fail to load.
package ext

import (
	"policycheck/internal/router"
	"policycheck/internal/router/ext/extensions/charmcli"
	"policycheck/internal/router/ext/extensions/prettystyle"
)

// optionalExtensions is the slice of capability extensions that extend the base router
// without adding dependencies to the core router code. These extensions load before
// application extensions during boot and are optional - boot continues with warnings
// if they fail to load.
var optionalExtensions = []router.Extension{
	&prettystyle.Extension{},
	&charmcli.Extension{},
}
