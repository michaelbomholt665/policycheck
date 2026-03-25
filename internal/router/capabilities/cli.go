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
)

// CLIOutputStyler styles CLI text and table output.
// Implementations translate the router's canonical semantic roles into their
// own renderer-specific style names, options, or formatting primitives.
type CLIOutputStyler interface {
	StyleText(kind string, input string) (string, error)
	StyleTable(headers []string, rows [][]string) (string, error)
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
