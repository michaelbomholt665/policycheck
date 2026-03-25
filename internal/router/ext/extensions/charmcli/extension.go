package charmcli

import (
	"fmt"
	"log"

	"policycheck/internal/router"
)

// Extension registers the optional CLI interaction capability.
type Extension struct{}

// Required reports that the charmcli extension is optional.
func (e *Extension) Required() bool {
	return false
}

// Consumes reports that the charmcli extension has no boot-time dependencies.
func (e *Extension) Consumes() []router.PortName {
	return nil
}

// Provides reports that the charmcli extension registers CLI chrome and interaction.
func (e *Extension) Provides() []router.PortName {
	return []router.PortName{router.PortCLIChrome, router.PortCLIInteraction}
}

// RouterProvideRegistration registers the CLI chrome and interaction provider into the boot registry.
func (e *Extension) RouterProvideRegistration(reg *router.Registry) error {
	provider := &Provider{}
	if err := reg.RouterRegisterProvider(router.PortCLIChrome, provider); err != nil {
		return fmt.Errorf("charmcli extension chrome: %w", err)
	}
	if err := reg.RouterRegisterProvider(router.PortCLIInteraction, provider); err != nil {
		return fmt.Errorf("charmcli extension interaction: %w", err)
	}

	log.Printf("charmcli extension initialized")
	return nil
}
