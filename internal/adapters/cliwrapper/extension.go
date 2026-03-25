// internal/adapters/cliwrapper/extension.go
package cliwrapper

import "policycheck/internal/router"

// Extension implements router.Extension for the CLI wrapper placeholder adapters.
//
// It registers all four wrapper-subsystem placeholder providers against their
// respective ports. Replace individual placeholder registrations as real
// implementations become available.
type Extension struct{}

// Required returns false — wrapper adapters degrade gracefully during development.
func (e *Extension) Required() bool { return false }

// Consumes returns nil — no boot-time dependencies.
func (e *Extension) Consumes() []router.PortName { return nil }

// Provides returns the four CLI wrapper ports registered by this extension.
func (e *Extension) Provides() []router.PortName {
	return []router.PortName{
		router.PortCLIWrapperDispatcher,
		router.PortCLIWrapperSecurityGate,
		router.PortCLIWrapperMacroRunner,
		router.PortCLIWrapperFormatter,
	}
}

// RouterProvideRegistration registers all four placeholder adapters.
func (e *Extension) RouterProvideRegistration(reg *router.Registry) error {
	if err := reg.RouterRegisterProvider(router.PortCLIWrapperDispatcher, NewDispatcherPlaceholder()); err != nil {
		return err
	}

	if err := reg.RouterRegisterProvider(router.PortCLIWrapperSecurityGate, NewSecurityGatePlaceholder()); err != nil {
		return err
	}

	if err := reg.RouterRegisterProvider(router.PortCLIWrapperMacroRunner, NewMacroRunnerPlaceholder()); err != nil {
		return err
	}

	return reg.RouterRegisterProvider(router.PortCLIWrapperFormatter, NewFormatterPlaceholder())
}

// ExtensionInstance returns the extension instance.
func ExtensionInstance() router.Extension {
	return &Extension{}
}
