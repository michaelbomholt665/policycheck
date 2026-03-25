// Package prettystyle Is a router capability extension that registers an optional
// CLI styling provider for router consumers.
//
// Usage:
//   - Depend on this extension when callers want styled CLI text or table output.
//   - Resolve router.PortCLIStyle through internal/router/capabilities.
//
// Package Concerns:
// - Required() must remain false because CLI styling is optional infrastructure.
// - Provides() must stay aligned with router.PortCLIStyle.
package prettystyle
