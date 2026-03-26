// internal/ports/scanners.go
// Declares the router port for scanner-backed policy fact extraction.
// Keeps scanner consumers independent from concrete language-specific adapters.
package ports

import (
	"context"
)

// ScannerProvider defines the contract for executing external scanners.
type ScannerProvider interface {
	// RunScanners executes the external scanners against the provided root directory
	// and returns the parsed policy facts.
	RunScanners(ctx context.Context, root string) ([]PolicyFact, error)

	// ScanFile executes the external scanners against a single file.
	ScanFile(ctx context.Context, root, path string) ([]PolicyFact, error)
}
