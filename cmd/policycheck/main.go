// cmd/policycheck/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"policycheck/internal/app"
	"policycheck/internal/policycheck/cli"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.BootPolicycheckApp(ctx); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "policycheck bootstrap failed: %v\n", err)
		os.Exit(1)
	}

	os.Exit(cli.Run(os.Args[1:]))
}
