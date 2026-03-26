// internal/ports/policy_types.go
// Defines shared scanner fact payloads that cross the router boundary.
// Keeps policy fact serialization stable across scanner providers and consumers.
package ports

// PolicyFact represents a fact extracted by the external scanner.
type PolicyFact struct {
	Kind              string   `json:"kind"`
	SymbolName        string   `json:"symbol_name"`
	Language          string   `json:"language"`
	SymbolKind        string   `json:"symbol_kind"`
	FilePath          string   `json:"file_path"`
	LineNumber        int      `json:"line_number"`
	EndLine           int      `json:"end_line"`
	Complexity        int      `json:"complexity"`
	ParamCount        int      `json:"param_count"`
	Params            []string `json:"params"`
	RepeatedNilGuards int      `json:"repeated_nil_guards"`
	Docstring         string   `json:"docstring"`
}
