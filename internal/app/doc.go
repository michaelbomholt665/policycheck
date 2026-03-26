// internal/app/doc.go
// Describes the app package's boot-time responsibilities for policycheck.
// Keeps package documentation aligned with the enforced file-header contract.
// Package app Provides application-level boot orchestration for the policycheck binary.
//
// Package Concerns:
// - Booting router-backed application capabilities before command execution.
// - Keeping entrypoint wiring isolated from feature-specific packages.
package app
