// Package main is the thin entry point for the policycheck binary.
// It passes embedded scanner assets to cli.Run and exits with the
// appropriate code. All business logic lives in internal/policycheck/.
//
// Package Concerns:
// - Embed scanner scripts (policy_scanner.py, policy_scanner.cjs) for subprocess execution.
// - Delegate all flag parsing, config loading, and policy checks to cli.Run.
package main
