package cliwrappercore

import "policycheck/internal/router"

// Extension registers the shared wrapper core provider.
type Extension struct{}

// Required reports that wrapper core is required once wrapper adapters are wired.
func (e *Extension) Required() bool { return true }

// Consumes reports no static router dependencies.
func (e *Extension) Consumes() []router.PortName { return nil }

// Provides reports the shared wrapper core port.
func (e *Extension) Provides() []router.PortName {
	return []router.PortName{router.PortCLIWrapperCore}
}

// RouterProvideRegistration registers the shared wrapper core provider.
func (e *Extension) RouterProvideRegistration(reg *router.Registry) error {
	return reg.RouterRegisterProvider(router.PortCLIWrapperCore, Provider{})
}
