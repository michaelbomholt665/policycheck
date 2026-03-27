// internal/router/capabilities/cli.go
// Defines the strictly typed router capabilities for CLI output styling,
// chrome rendering, and interactive prompting.

package capabilities

import (
	"fmt"

	"policycheck/internal/router"
)

const (
	// TextKindPlain leaves the input unchanged.
	// Providers should treat these values as canonical semantic roles and map them
	// into the native styling model of the renderer they wrap.
	TextKindPlain = "plain"
	// TextKindHeader emphasizes headings.
	TextKindHeader = "header"
	// TextKindDebug highlights verbose background routing details.
	TextKindDebug = "debug"
	// TextKindInfo highlights informational output.
	TextKindInfo = "info"
	// TextKindSuccess highlights successful output.
	TextKindSuccess = "success"
	// TextKindWarning highlights warning output.
	TextKindWarning = "warning"
	// TextKindError highlights error output.
	TextKindError = "error"
	// TextKindFatal highlights unrecoverable state or crash output.
	TextKindFatal = "fatal"
	// TextKindMuted de-emphasizes secondary output.
	TextKindMuted = "muted"

	// TableKindNormal renders a standard bordered table.
	TableKindNormal = "table.normal"
	// TableKindCompact renders a compact table with minimal separators.
	TableKindCompact = "table.compact"
	// TableKindMerged renders a table with vertical merging in the first column.
	TableKindMerged = "table.merged"
	// TableKindMuted renders a de-emphasized table.
	TableKindMuted = "table.muted"

	// LayoutKindPanel renders a bordered container with a title.
	LayoutKindPanel = "layout.panel"
	// LayoutKindSplit renders content blocks side by side.
	LayoutKindSplit = "layout.split"
	// LayoutKindGutter renders a fixed-width status gutter followed by content.
	LayoutKindGutter = "layout.gutter"

	// PromptKindSelect renders a single-choice prompt.
	PromptKindSelect = "prompt.select"
	// PromptKindToggle renders a multi-choice toggle prompt.
	PromptKindToggle = "prompt.toggle"
	// PromptKindConfirm renders a yes/no confirmation prompt.
	PromptKindConfirm = "prompt.confirm"
	// PromptKindInput renders a free-text input prompt.
	PromptKindInput = "prompt.input"
)

// Choice maps business keys to human labels for interactive prompts.
type Choice struct {
	Key   string
	Label string
}

// CLIOutputStyler styles CLI text, tables, and layouts.
// Implementations translate the router's canonical semantic roles into their
// own renderer-specific style names, options, or formatting primitives.
type CLIOutputStyler interface {
	StyleText(kind string, input string) (string, error)
	StyleTable(kind string, headers []string, rows [][]string) (string, error)
	StyleLayout(kind string, title string, content ...string) (string, error)
}

// CLIChromeStyler styles semantic CLI text and layouts without owning table rendering.
type CLIChromeStyler interface {
	StyleText(kind string, input string) (string, error)
	StyleLayout(kind string, title string, content ...string) (string, error)
}

// CLIInteractor handles interactive prompt flows.
type CLIInteractor interface {
	StylePrompt(kind string, title string, description string, options []Choice) (any, error)
}

// ResolveCLIOutputStyler resolves the router-native CLI styling capability.
func ResolveCLIOutputStyler() (CLIOutputStyler, error) {
	provider, err := router.RouterResolveProvider(router.PortCLIStyle)
	if err != nil {
		return nil, err
	}

	styler, ok := provider.(CLIOutputStyler)
	if !ok {
		return nil, &router.RouterError{
			Code: router.PortContractMismatch,
			Port: router.PortCLIStyle,
			Err:  fmt.Errorf("provider %T does not implement capabilities.CLIOutputStyler", provider),
		}
	}

	return styler, nil
}

// ResolveCLIChromeStyler resolves the router-native CLI chrome capability.
func ResolveCLIChromeStyler() (CLIChromeStyler, error) {
	provider, err := router.RouterResolveProvider(router.PortCLIChrome)
	if err != nil {
		return nil, err
	}

	chromeStyler, ok := provider.(CLIChromeStyler)
	if !ok {
		return nil, &router.RouterError{
			Code: router.PortContractMismatch,
			Port: router.PortCLIChrome,
			Err:  fmt.Errorf("provider %T does not implement capabilities.CLIChromeStyler", provider),
		}
	}

	return chromeStyler, nil
}

// ResolveCLIInteractor resolves the router-native CLI interaction capability.
func ResolveCLIInteractor() (CLIInteractor, error) {
	provider, err := router.RouterResolveProvider(router.PortCLIInteraction)
	if err != nil {
		return nil, err
	}

	interactor, ok := provider.(CLIInteractor)
	if !ok {
		return nil, &router.RouterError{
			Code: router.PortContractMismatch,
			Port: router.PortCLIInteraction,
			Err:  fmt.Errorf("provider %T does not implement capabilities.CLIInteractor", provider),
		}
	}

	return interactor, nil
}
