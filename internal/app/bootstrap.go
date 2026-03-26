// internal/app/bootstrap.go
// Boots the policycheck application against the router-backed extension bundle.
// Keeps startup sequencing isolated from feature packages and command handlers.
package app

import (
	"context"
	"fmt"

	"policycheck/internal/router/ext"
)

// BootPolicycheckApp boots the router for the policycheck application.
func BootPolicycheckApp(ctx context.Context) error {
	warnings, err := ext.RouterBootExtensions(ctx)
	if err != nil {
		return fmt.Errorf("router boot failed: %w", err)
	}

	for _, warning := range warnings {
		fmt.Printf("router warning: %v\n", warning)
	}

	return nil
}
