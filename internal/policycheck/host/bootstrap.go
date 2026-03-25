// internal/policycheck/host/bootstrap.go
package host

import (
	"context"
	"fmt"

	"policycheck/internal/router/ext"
)

// BootPolicycheckHost boots the router for the policycheck application.
func BootPolicycheckHost(ctx context.Context) error {
	warnings, err := ext.RouterBootExtensions(ctx)
	if err != nil {
		return fmt.Errorf("router boot failed: %w", err)
	}

	// Just log warnings if there are any optional capabilities that fail to boot.
	for _, w := range warnings {
		fmt.Printf("router warning: %v\n", w)
	}

	return nil
}
