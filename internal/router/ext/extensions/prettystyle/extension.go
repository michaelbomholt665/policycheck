// internal/router/ext/extensions/prettystyle/extension.go
// Defines the strictly typed router extension for the prettystyle capability.

package prettystyle

import (
	"fmt"
	"log"

	"policycheck/internal/router"
)

// Extension registers the optional CLI styling capability.
type Extension struct{}

// Required reports that the prettystyle extension is optional.
func (e *Extension) Required() bool {
	return false
}

// Consumes reports that the prettystyle extension has no boot-time dependencies.
func (e *Extension) Consumes() []router.PortName {
	return nil
}

// Provides reports that the prettystyle extension registers router.PortCLIStyle.
func (e *Extension) Provides() []router.PortName {
	return []router.PortName{router.PortCLIStyle}
}

// RouterProvideRegistration registers the CLI styling provider into the boot registry.
func (e *Extension) RouterProvideRegistration(reg *router.Registry) error {
	provider := &Provider{}
	if err := reg.RouterRegisterProvider(router.PortCLIStyle, provider); err != nil {
		return fmt.Errorf("prettystyle extension: %w", err)
	}

	log.Printf("prettystyle extension initialized")
	return nil
}
