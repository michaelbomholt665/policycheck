// Package prettystyle is a router capability extension that registers an optional
// CLI output provider for router consumers.
//
// Usage:
//   - Depend on this extension when callers want styled CLI text, tables, or
//     simple semantic layouts.
//   - Resolve router.PortCLIStyle through internal/router/capabilities.
//
// Package Concerns:
// - Required() must remain false because CLI styling is optional infrastructure.
// - Provides() must stay aligned with router.PortCLIStyle.
package prettystyle
