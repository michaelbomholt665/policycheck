package scanners

import _ "embed"

//go:embed policy_scanner.py
var policyScannerPy []byte

//go:embed policy_scanner.cjs
var policyScannerJS []byte

//go:embed policy_gate_default.toml
var policyGateDefaultTemplate []byte

// EmbeddedPolicyGateDefaultTemplate returns the embedded default policy template.
func EmbeddedPolicyGateDefaultTemplate() []byte {
	return append([]byte(nil), policyGateDefaultTemplate...)
}
