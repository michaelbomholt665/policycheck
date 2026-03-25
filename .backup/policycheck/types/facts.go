// internal/policycheck/types/facts.go
// Defines types for function quality facts emitted by external scanners.

package types

const ScopeProjectRepo = true

// PolicyFact is a quality metric emitted by a language scanner for a single function or method.
type PolicyFact struct {
	Kind       string `json:"kind"`
	Language   string `json:"language"`
	Path       string `json:"path"`
	Name       string `json:"name"`
	Line       int    `json:"line"`
	EndLine    int    `json:"end_line"`
	LOC        int    `json:"loc"`
	CTX        int    `json:"ctx"`
	SymbolKind string `json:"symbol_kind,omitempty"`
}

// PolicyScannerAssets holds the file paths to materialized scanner scripts on disk.
type PolicyScannerAssets struct {
	RootDir string
	Python  string
	TS      string
}

// ScannerBytes carries the raw embedded content for the external scanner scripts.
// It is passed from the cmd entry point into the embedded package at runtime.
type ScannerBytes struct {
	Python []byte
	JS     []byte
}
