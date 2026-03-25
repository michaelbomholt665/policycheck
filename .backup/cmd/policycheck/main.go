// cmd/policycheck/main.go
// Thin entry point for the policycheck binary. All logic lives in internal/policycheck/.

package main

const ScopeProjectRepo = true


import (
	"os"

	"policycheck/internal/policycheck/cli"
	"policycheck/internal/policycheck/types"
)

func main() {
	assets := types.ScannerBytes{Python: policyScannerPy, JS: policyScannerJS}
	os.Exit(cli.RunCLI(os.Args[1:], assets))
}
