// internal/adapters/cliwrapper/wrapperboot.go
package cliwrapper

import (
	"errors"
	"fmt"

	"policycheck/internal/ports"
	"policycheck/internal/router"
)

// WrapperResolved holds the four wrapper-subsystem capabilities resolved from the router.
//
// All fields may be placeholder implementations during development. Callers should
// treat a non-nil WrapperResolved as the wrapper entry being operational at the
// placeholder level; actual functionality arrives when real adapters replace the
// placeholder registrations.
type WrapperResolved struct {
	// Dispatcher is the resolved CLI command dispatcher for the wrapper subsystem.
	Dispatcher ports.CLIWrapperDispatcher
	// SecurityGate is the resolved package-security capability for the wrapper subsystem.
	SecurityGate ports.CLIWrapperSecurityGate
	// MacroRunner is the resolved macro execution capability for the wrapper subsystem.
	MacroRunner ports.CLIWrapperMacroRunner
	// Formatter is the resolved output-formatting capability for the wrapper subsystem.
	Formatter ports.CLIWrapperFormatter
}

// WrapperBootEntry resolves all four wrapper-subsystem ports from the router boundary.
//
// WrapperBootEntry is the wrapper dispatch seam: it reaches the wrapper capabilities
// without touching any policycheck command execution path. Call this function from
// any wrapper entry point (CLI flag branch, dedicated subcommand, or integration test)
// to prove that the wrapper subsystem boots independently.
//
// Returns an error if the router has not been booted or if a required port is
// missing from the registry.
func WrapperBootEntry() (WrapperResolved, error) {
	dispatcher, err := resolveWrapperDispatcher()
	if err != nil {
		return WrapperResolved{}, fmt.Errorf("wrapper boot entry: resolve dispatcher: %w", err)
	}

	securityGate, err := resolveWrapperSecurityGate()
	if err != nil {
		return WrapperResolved{}, fmt.Errorf("wrapper boot entry: resolve security gate: %w", err)
	}

	macroRunner, err := resolveWrapperMacroRunner()
	if err != nil {
		return WrapperResolved{}, fmt.Errorf("wrapper boot entry: resolve macro runner: %w", err)
	}

	formatter, err := resolveWrapperFormatter()
	if err != nil {
		return WrapperResolved{}, fmt.Errorf("wrapper boot entry: resolve formatter: %w", err)
	}

	return WrapperResolved{
		Dispatcher:   dispatcher,
		SecurityGate: securityGate,
		MacroRunner:  macroRunner,
		Formatter:    formatter,
	}, nil
}

func resolveWrapperDispatcher() (ports.CLIWrapperDispatcher, error) {
	provider, err := router.RouterResolveProvider(router.PortCLIWrapperDispatcher)
	if err != nil {
		return nil, err
	}

	d, ok := provider.(ports.CLIWrapperDispatcher)
	if !ok {
		return nil, errors.New("provider does not implement CLIWrapperDispatcher")
	}

	return d, nil
}

func resolveWrapperSecurityGate() (ports.CLIWrapperSecurityGate, error) {
	provider, err := router.RouterResolveProvider(router.PortCLIWrapperSecurityGate)
	if err != nil {
		return nil, err
	}

	s, ok := provider.(ports.CLIWrapperSecurityGate)
	if !ok {
		return nil, errors.New("provider does not implement CLIWrapperSecurityGate")
	}

	return s, nil
}

func resolveWrapperMacroRunner() (ports.CLIWrapperMacroRunner, error) {
	provider, err := router.RouterResolveProvider(router.PortCLIWrapperMacroRunner)
	if err != nil {
		return nil, err
	}

	m, ok := provider.(ports.CLIWrapperMacroRunner)
	if !ok {
		return nil, errors.New("provider does not implement CLIWrapperMacroRunner")
	}

	return m, nil
}

func resolveWrapperFormatter() (ports.CLIWrapperFormatter, error) {
	provider, err := router.RouterResolveProvider(router.PortCLIWrapperFormatter)
	if err != nil {
		return nil, err
	}

	f, ok := provider.(ports.CLIWrapperFormatter)
	if !ok {
		return nil, errors.New("provider does not implement CLIWrapperFormatter")
	}

	return f, nil
}
