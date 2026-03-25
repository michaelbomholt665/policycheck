package router

// PortName is a typed router port identifier.
type PortName string

// Provider is the registered implementation for a router port.
type Provider any

const (
	// PortPrimary is an example primary provider port.
	PortPrimary PortName = "primary"
	// PortSecondary is an example secondary provider port.
	PortSecondary PortName = "secondary"
	// PortTertiary is an example tertiary provider port.
	PortTertiary PortName = "tertiary"
	// PortOptional is an example optional provider port.
	PortOptional PortName = "optional"
	// PortCLIStyle is the router-native CLI styling capability port.
	PortCLIStyle PortName = "cli-style"
	PortConfig   PortName = "config"
	PortWalk     PortName = "walk"
	PortScanner  PortName = "scanner"
	PortReadFile PortName = "readfile"
)
