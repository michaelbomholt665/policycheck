// internal/router/ext/extensions/charmcli/doc.go
// Provides documentation for the optional CLI interaction provider extension.

// Package charmcli is a router capability extension that registers an optional
// CLI chrome and interaction provider backed by Charm libraries.
//
// Usage:
//   - Depend on this extension when callers need semantic CLI text or layout
//     chrome such as panels, splits, or gutters.
//   - Depend on this extension when callers need semantic CLI prompts such as
//     select, confirm, toggle, or text input flows.
//   - Resolve router.PortCLIChrome and router.PortCLIInteraction through
//     internal/router/capabilities.
//
// Package Concerns:
//   - Required() must remain false because CLI chrome and interaction are optional infrastructure.
//   - Provides() must stay aligned with router.PortCLIChrome and router.PortCLIInteraction.
package charmcli
