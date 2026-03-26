// internal/ports/scanners.go
package ports

import (
	"context"
)

// ScannerProvider defines the contract for executing external scanners.
type ScannerProvider interface {
	// RunScanners executes the external scanners against the provided root directory
	// and returns the parsed policy facts.
	RunScanners(ctx context.Context, root string) ([]PolicyFact, error)
}
