// internal/policycheck/cli/capabilities.go
// Package cli provides CLI-specific capabilities and rendering helpers.
// It manages the resolution and use of optional router-native stylers and
// interactors for the policycheck command line interface.
package cli

import (
	"errors"
	"fmt"
	"os"

	"policycheck/internal/router"
	"policycheck/internal/router/capabilities"
)

// Renderers contains the router-native CLI capabilities policycheck can use.
type Renderers struct {
	Output     capabilities.CLIOutputStyler
	Chrome     capabilities.CLIChromeStyler
	Interactor capabilities.CLIInteractor
}

// ResolveRenderers resolves the optional router-native CLI capabilities.
func ResolveRenderers() (Renderers, error) {
	output, err := resolveOptionalOutputStyler()
	if err != nil {
		return Renderers{}, err
	}

	chrome, err := resolveOptionalChromeStyler()
	if err != nil {
		return Renderers{}, err
	}

	interactor, err := resolveOptionalInteractor()
	if err != nil {
		return Renderers{}, err
	}

	return Renderers{
		Output:     output,
		Chrome:     chrome,
		Interactor: interactor,
	}, nil
}

// resolveOptionalOutputStyler resolves the CLIOutputStyler from the router.
func resolveOptionalOutputStyler() (capabilities.CLIOutputStyler, error) {
	styler, err := capabilities.ResolveCLIOutputStyler()
	if err == nil || isOptionalCapabilityUnavailable(err) {
		return styler, nil
	}

	return nil, fmt.Errorf("resolve CLI output styler: %w", err)
}

// resolveOptionalChromeStyler resolves the CLIChromeStyler from the router.
func resolveOptionalChromeStyler() (capabilities.CLIChromeStyler, error) {
	styler, err := capabilities.ResolveCLIChromeStyler()
	if err == nil || isOptionalCapabilityUnavailable(err) {
		return styler, nil
	}

	return nil, fmt.Errorf("resolve CLI chrome styler: %w", err)
}

// resolveOptionalInteractor resolves the CLIInteractor from the router.
func resolveOptionalInteractor() (capabilities.CLIInteractor, error) {
	interactor, err := capabilities.ResolveCLIInteractor()
	if err == nil || isOptionalCapabilityUnavailable(err) {
		return interactor, nil
	}

	return nil, fmt.Errorf("resolve CLI interactor: %w", err)
}

// isOptionalCapabilityUnavailable reports whether err indicates a missing router port.
func isOptionalCapabilityUnavailable(err error) bool {
	var routerErr *router.RouterError
	if !errors.As(err, &routerErr) {
		return false
	}

	return routerErr.Code == router.PortNotFound
}

// styleChromeText applies a chrome style to the given input string.
func styleChromeText(chrome capabilities.CLIChromeStyler, kind, input string) (string, error) {
	if chrome == nil {
		return input, nil
	}

	styled, err := chrome.StyleText(kind, input)
	if err != nil {
		return "", fmt.Errorf("style chrome text %q: %w", kind, err)
	}

	return styled, nil
}

// styleChromeLayout applies a chrome layout style to the given title and content.
func styleChromeLayout(chrome capabilities.CLIChromeStyler, kind, title string, content ...string) (string, error) {
	if chrome == nil {
		return renderPlainLayout(title, content...), nil
	}

	styled, err := chrome.StyleLayout(kind, title, content...)
	if err != nil {
		return "", fmt.Errorf("style chrome layout %q: %w", kind, err)
	}

	return styled, nil
}

// printRouterWarnings styles and prints router-provider resolution warnings to stderr.
func printRouterWarnings(chrome capabilities.CLIChromeStyler, warnings []error) {
	for _, warningErr := range warnings {
		if warningErr == nil {
			continue
		}

		message := fmt.Sprintf("router warning: %v", warningErr)
		styled, err := styleChromeText(chrome, capabilities.TextKindWarning, message)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s\n", message)
			continue
		}

		fmt.Fprintln(os.Stderr, styled)
	}
}

// printInteractiveFallbackNotice prints a warning when the interactive capability is missing.
func printInteractiveFallbackNotice(chrome capabilities.CLIChromeStyler) {
	message := "interactive CLI capability unavailable; falling back to static rule descriptions"
	styled, err := styleChromeText(chrome, capabilities.TextKindMuted, message)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %s\n", message)
		return
	}

	fmt.Fprintln(os.Stderr, styled)
}
