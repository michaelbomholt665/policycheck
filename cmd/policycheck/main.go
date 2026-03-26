// cmd/policycheck/main.go
// Package main is the entry point for the policycheck CLI application.
// It bootstraps the application infrastructure and dispatches to the
// primary CLI runner.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"policycheck/internal/app"
	"policycheck/internal/policycheck/cli"
)

// main is the entry point for the policycheck binary.
//
// It initializes the application context, bootstraps the host
// infrastructure, and executes the CLI dispatch logic.
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.BootPolicycheckApp(ctx); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "policycheck bootstrap failed: %v\n", err)
		os.Exit(1)
	}

	os.Exit(cli.Run(os.Args[1:]))
}
