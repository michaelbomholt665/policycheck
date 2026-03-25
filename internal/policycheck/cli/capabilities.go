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

func resolveOptionalOutputStyler() (capabilities.CLIOutputStyler, error) {
	styler, err := capabilities.ResolveCLIOutputStyler()
	if err == nil || isOptionalCapabilityUnavailable(err) {
		return styler, nil
	}

	return nil, fmt.Errorf("resolve CLI output styler: %w", err)
}

func resolveOptionalChromeStyler() (capabilities.CLIChromeStyler, error) {
	styler, err := capabilities.ResolveCLIChromeStyler()
	if err == nil || isOptionalCapabilityUnavailable(err) {
		return styler, nil
	}

	return nil, fmt.Errorf("resolve CLI chrome styler: %w", err)
}

func resolveOptionalInteractor() (capabilities.CLIInteractor, error) {
	interactor, err := capabilities.ResolveCLIInteractor()
	if err == nil || isOptionalCapabilityUnavailable(err) {
		return interactor, nil
	}

	return nil, fmt.Errorf("resolve CLI interactor: %w", err)
}

func isOptionalCapabilityUnavailable(err error) bool {
	var routerErr *router.RouterError
	if !errors.As(err, &routerErr) {
		return false
	}

	return routerErr.Code == router.PortNotFound
}

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

func printInteractiveFallbackNotice(chrome capabilities.CLIChromeStyler) {
	message := "interactive CLI capability unavailable; falling back to static rule descriptions"
	styled, err := styleChromeText(chrome, capabilities.TextKindMuted, message)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %s\n", message)
		return
	}

	fmt.Fprintln(os.Stderr, styled)
}
