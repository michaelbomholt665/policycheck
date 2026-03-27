// internal/router/ext/extensions/telemetry/doc.go
// Provides documentation for the optional telemetry capabilities extension.

// Package telemetry is a router capability extension that registers an optional
// telemetry provider before application extensions boot.
//
// Usage:
//   - Depend on this extension when application extensions need an optional
//     cross-cutting capability before app-specific adapters boot.
//   - Consume router.PortOptional from an application extension only when the
//     adapter can degrade gracefully if telemetry is unavailable.
//
// Package Concerns:
//   - Implements router.Extension only; no imports from internal/adapters or internal/ports.
//   - Registering under router.PortOptional makes this capability available to any
//     application extension that declares it in Consumes().
//   - Failure to boot results in an OptionalExtensionFailed warning; boot continues.
package telemetry
