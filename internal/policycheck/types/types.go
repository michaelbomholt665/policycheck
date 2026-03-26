// internal/policycheck/types/types.go
package types

import "policycheck/internal/ports"

// PolicyFact represents a fact extracted by the external scanner.
type PolicyFact = ports.PolicyFact

// Violation represents a policy check violation.
type Violation struct {
	RuleID   string `json:"rule_id"`
	File     string `json:"file"`
	Function string `json:"function"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
	Line     int    `json:"line"`
}
