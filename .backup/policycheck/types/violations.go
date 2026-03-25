// internal/policycheck/types/violations.go
// Defines the core violation, finding, and scan-context types for policy checks.

package types

const ScopeProjectRepo = true

// Violation represents a single policy violation with its location and description.
type Violation struct {
	Path     string
	Message  string
	Severity string
}

// Finding is the structured output representation of a policy violation
// used for JSON and NDJSON output formats.
type Finding struct {
	Kind     string `json:"kind"`
	Severity string `json:"severity,omitempty"`
	Path     string `json:"path"`
	Message  string `json:"message"`
}

// FileLOCResult holds warning and violation pointers from a single file LOC evaluation.
type FileLOCResult struct {
	Warning   *Violation
	Violation *Violation
}

// CommandHandlerAnalysis records the result of inspecting a command handler function
// for CLI formatter compliance.
type CommandHandlerAnalysis struct {
	Name           string
	HasFormatter   bool
	HasRawFmtPrint bool
}
