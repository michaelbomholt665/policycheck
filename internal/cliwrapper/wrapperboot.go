// internal/cliwrapper/wrapperboot.go
// Resolves wrapper subsystem capabilities from the router boundary at startup.
// Keeps command consumers decoupled from concrete adapter implementations.
package cliwrapper

import (
	"errors"
	"fmt"

	"policycheck/internal/ports"
	"policycheck/internal/router"
)

// WrapperResolved holds the four wrapper-subsystem capabilities resolved from the router.
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

// resolveWrapperDispatcher resolves the wrapper dispatcher contract from the router.
func resolveWrapperDispatcher() (ports.CLIWrapperDispatcher, error) {
	provider, err := router.RouterResolveProvider(router.PortCLIWrapperDispatcher)
	if err != nil {
		return nil, fmt.Errorf("resolve CLIWrapperDispatcher: %w", err)
	}

	dispatcher, ok := provider.(ports.CLIWrapperDispatcher)
	if !ok {
		return nil, errors.New("provider does not implement CLIWrapperDispatcher")
	}

	return dispatcher, nil
}

// resolveWrapperSecurityGate resolves the wrapper security gate contract from the router.
func resolveWrapperSecurityGate() (ports.CLIWrapperSecurityGate, error) {
	provider, err := router.RouterResolveProvider(router.PortCLIWrapperSecurityGate)
	if err != nil {
		return nil, fmt.Errorf("resolve CLIWrapperSecurityGate: %w", err)
	}

	securityGate, ok := provider.(ports.CLIWrapperSecurityGate)
	if !ok {
		return nil, errors.New("provider does not implement CLIWrapperSecurityGate")
	}

	return securityGate, nil
}

// resolveWrapperMacroRunner resolves the wrapper macro runner contract from the router.
func resolveWrapperMacroRunner() (ports.CLIWrapperMacroRunner, error) {
	provider, err := router.RouterResolveProvider(router.PortCLIWrapperMacroRunner)
	if err != nil {
		return nil, fmt.Errorf("resolve CLIWrapperMacroRunner: %w", err)
	}

	macroRunner, ok := provider.(ports.CLIWrapperMacroRunner)
	if !ok {
		return nil, errors.New("provider does not implement CLIWrapperMacroRunner")
	}

	return macroRunner, nil
}

// resolveWrapperFormatter resolves the wrapper formatter contract from the router.
func resolveWrapperFormatter() (ports.CLIWrapperFormatter, error) {
	provider, err := router.RouterResolveProvider(router.PortCLIWrapperFormatter)
	if err != nil {
		return nil, fmt.Errorf("resolve CLIWrapperFormatter: %w", err)
	}

	formatter, ok := provider.(ports.CLIWrapperFormatter)
	if !ok {
		return nil, errors.New("provider does not implement CLIWrapperFormatter")
	}

	return formatter, nil
}
