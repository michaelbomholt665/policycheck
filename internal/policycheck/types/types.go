// internal/policycheck/types/types.go
package types

// PolicyFact represents a fact extracted by the external scanner.
type PolicyFact struct {
	Kind              string `json:"kind"`
	Version           string `json:"version"`
	SymbolName        string `json:"symbol_name"`
	QualifiedName     string `json:"qualified_name"`
	Language          string `json:"language"`
	SymbolKind        string `json:"symbol_kind"`
	FilePath          string `json:"file_path"`
	LineNumber        int    `json:"line_number"`
	EndLine           int    `json:"end_line"`
	Complexity        int    `json:"complexity"`
	ParamCount        int    `json:"param_count"`
	RepeatedNilGuards int    `json:"repeated_nil_guards"`
	SignatureText     string `json:"signature_text"`
	ObservationHash   string `json:"observation_hash"`
}

// Violation represents a policy check violation.
type Violation struct {
	RuleID      string `json:"rule_id"`
	File        string `json:"file"`
	Function    string `json:"function"`
	Message     string `json:"message"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // "error" or "warn"
	Line        int    `json:"line"`
}
