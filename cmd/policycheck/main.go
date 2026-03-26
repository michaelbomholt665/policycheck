// cmd/policycheck/main.go
// Package main is the entry point for the policycheck CLI application.
// It bootstraps the application infrastructure and dispatches to the
// shared binary dispatch seam.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"policycheck/internal/app"
)

// main is the entry point for the policycheck binary.
//
// It initializes the application context, bootstraps the host
// infrastructure, and executes the shared CLI dispatch logic.
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.BootPolicycheckApp(ctx); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "policycheck bootstrap failed: %v\n", err)
		os.Exit(1)
	}

	os.Exit(app.Run(ctx, os.Args[1:]))
}
