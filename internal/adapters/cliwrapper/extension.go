// internal/adapters/cliwrapper/extension.go
package cliwrapper

import (
	"policycheck/internal/router"
)

// defaultSecurityThreshold is the OSV block threshold used by the real dispatcher
// when no repo-level config is loaded at extension boot time.
const defaultSecurityThreshold = SeverityHigh

// Extension implements router.Extension for the CLI wrapper subsystem.
//
// The dispatcher and security-gate ports are backed by real implementations.
// MacroRunner and Formatter remain placeholder registrations until their
// respective TDD phases are complete.
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

// RouterProvideRegistration registers the real dispatcher and the remaining
// placeholder adapters for the wrapper subsystem ports.
func (e *Extension) RouterProvideRegistration(reg *router.Registry) error {
	osvGate := NewOSVSecurityAdapter(defaultSecurityThreshold)
	dispatcher := NewWrapperDispatcher(WrapperConfig{}, defaultExecFunc)

	if err := reg.RouterRegisterProvider(router.PortCLIWrapperDispatcher, dispatcher); err != nil {
		return err
	}

	if err := reg.RouterRegisterProvider(router.PortCLIWrapperSecurityGate, osvGate); err != nil {
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
