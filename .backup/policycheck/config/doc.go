// Package config Handles loading, defaulting, and validation of the policy-gate.toml configuration.
// It is the single source of truth for all policycheck tuning parameters.
//
// Package Concerns:
// - Load and validate policy configuration from TOML files.
// - Provide sensible defaults and a template file for first-run bootstrap.
package config
