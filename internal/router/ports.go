package router

// PortName is a typed router port identifier.
type PortName string

// Provider is the registered implementation for a router port.
type Provider any

const (
	PortPrimary                PortName = "primary"
	PortSecondary              PortName = "secondary"
	PortTertiary               PortName = "tertiary"
	PortOptional               PortName = "optional"
	PortCLIStyle               PortName = "cli-style"
	PortCLIChrome              PortName = "cli-chrome"
	PortCLIInteraction         PortName = "cli-interaction"
	PortConfig                 PortName = "config"
	PortWalk                   PortName = "walk"
	PortScanner                PortName = "scanner"
	PortReadFile               PortName = "readfile"
	PortCLIWrapperCore         PortName = "cli-wrapper-core"
	PortCLIWrapperDispatcher   PortName = "cli-wrapper-dispatcher"
	PortCLIWrapperSecurityGate PortName = "cli-wrapper-security"
	PortCLIWrapperMacroRunner  PortName = "cli-wrapper-macro"
	PortCLIWrapperFormatter    PortName = "cli-wrapper-format"
)
