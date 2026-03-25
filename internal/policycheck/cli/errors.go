// internal/policycheck/cli/errors.go
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
