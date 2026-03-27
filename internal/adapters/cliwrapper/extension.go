// internal/adapters/cliwrapper/extension.go
package cliwrapper

import "policycheck/internal/router"

// Extension implements router.Extension for the CLI wrapper subsystem.
//
// All wrapper ports are backed by real adapters. Cross-capability dependencies
// such as repository walking are resolved at runtime through the router boundary.
type Extension struct{}

// Required returns false — wrapper adapters degrade gracefully during development.
func (e *Extension) Required() bool { return false }

// Consumes returns nil because wrapper cross-capability dependencies are resolved at runtime.
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

// RouterProvideRegistration registers the real wrapper adapters for each subsystem port.
func (e *Extension) RouterProvideRegistration(reg *router.Registry) error {
	osvGate := NewOSVSecurityAdapter(WrapperSecurityConfig{})
	dispatcher := NewWrapperDispatcher(WrapperConfig{}, defaultExecFunc)
	dispatcher.loadConfig = loadActiveDispatcherConfig

	if err := reg.RouterRegisterProvider(router.PortCLIWrapperDispatcher, dispatcher); err != nil {
		return err
	}

	if err := reg.RouterRegisterProvider(router.PortCLIWrapperSecurityGate, osvGate); err != nil {
		return err
	}

	if err := reg.RouterRegisterProvider(router.PortCLIWrapperMacroRunner, NewMacroRunnerAdapter()); err != nil {
		return err
	}

	return reg.RouterRegisterProvider(router.PortCLIWrapperFormatter, NewHeaderFormatterAdapter())
}

// ExtensionInstance returns the extension instance.
func ExtensionInstance() router.Extension {
	return &Extension{}
}
