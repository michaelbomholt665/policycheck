// cmd/policycheck/main.go
package main

import (
	"os"

	"policycheck/internal/policycheck/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
