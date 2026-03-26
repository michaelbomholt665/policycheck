// Package app provides application-level boot orchestration for the policycheck binary.
//
// Package Concerns:
// - Booting router-backed application capabilities before command execution.
// - Keeping entrypoint wiring isolated from feature-specific packages.
package app
