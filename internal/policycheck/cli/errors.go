// internal/policycheck/cli/errors.go
// Package cli/errors handles top-level CLI error reporting and exit code mapping.
// It provides consistent formatting for errors sent to the user's terminal.
package cli

import (
	"fmt"
	"os"
)

// HandleError prints an error message to stderr and returns the appropriate exit code.
func HandleError(err error) int {
	if err == nil {
		return 0
	}

	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	return 1
}
