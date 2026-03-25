// cmd/policycheck/embed.go
// Provides access to embedded assets, including the Python and TypeScript scanners.

package main

const ScopeProjectRepo = true


import _ "embed"

//go:embed policy_scanner.py
var policyScannerPy []byte

//go:embed dist/policy_scanner.cjs
var policyScannerJS []byte
