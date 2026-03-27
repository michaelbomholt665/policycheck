// internal/adapters/cliwrapper/core.go
package cliwrapper

import (
	"fmt"

	"policycheck/internal/ports"
	"policycheck/internal/router"
)

func resolveWrapperCore() (ports.CLIWrapperCore, error) {
	raw, err := router.RouterResolveProvider(router.PortCLIWrapperCore)
	if err != nil {
		return nil, fmt.Errorf("resolve CLIWrapperCore: %w", err)
	}

	core, ok := raw.(ports.CLIWrapperCore)
	if !ok {
		return nil, fmt.Errorf("provider does not implement CLIWrapperCore")
	}

	return core, nil
}
