// internal\router\ext\extensions\telemetry\extension.go

package telemetry

import (
	"fmt"
	"log"

	"policycheck/internal/router"
)

// Extension is the optional router capability extension for telemetry.
// It satisfies router.Extension and registers a telemetry provider under
// router.PortOptional before application extensions boot.
type Extension struct{}

// Required reports that the telemetry extension is optional.
// A boot failure produces an OptionalExtensionFailed warning; boot continues.
func (e *Extension) Required() bool {
	return false
}

// Consumes reports that the telemetry extension has no boot-time port dependencies.
func (e *Extension) Consumes() []router.PortName {
	return nil
}

// Provides reports that the telemetry extension registers router.PortOptional.
func (e *Extension) Provides() []router.PortName {
	return []router.PortName{router.PortOptional}
}

// RouterProvideRegistration registers the telemetry provider into the boot registry.
func (e *Extension) RouterProvideRegistration(reg *router.Registry) error {
	provider := struct{ Name string }{Name: "telemetry-provider"}

	if err := reg.RouterRegisterProvider(router.PortOptional, provider); err != nil {
		return fmt.Errorf("telemetry extension: %w", err)
	}

	log.Println("telemetry extension initialized")

	return nil
}
