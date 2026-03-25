// Package core Implements all policy check logic for the policycheck engine.
// Each check function is independent and receives only the data it needs.
//
// Package Concerns:
// - Implement all policy check categories (quality, security, architecture, hygiene, contracts, topology).
// - Orchestrate all checks through RunPolicyChecks and return typed results.
package core
