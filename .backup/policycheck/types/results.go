// internal/policycheck/types/results.go
// Defines the aggregate result types and function quality warning types.

package types

const ScopeProjectRepo = true

// PolicyCheckResults aggregates all outputs from a full policy check run.
type PolicyCheckResults struct {
	ScannerErrors []Violation
	Warnings      []Violation
	Violations    []Violation
}

// FunctionQualityLevel classifies the severity of a function quality warning.
type FunctionQualityLevel int

const (
	// FunctionQualityLevelMild indicates mildly elevated complexity.
	FunctionQualityLevelMild FunctionQualityLevel = iota + 1
	// FunctionQualityLevelElevated indicates significantly elevated complexity.
	FunctionQualityLevelElevated
	// FunctionQualityLevelImmediate indicates complexity requiring immediate refactoring.
	FunctionQualityLevelImmediate
)

// FunctionQualityWarning carries the details of a complexity or LOC warning for a single function.
type FunctionQualityWarning struct {
	Path     string
	Function string
	Message  string
	Level    FunctionQualityLevel
}
