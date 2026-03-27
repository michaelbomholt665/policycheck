package router

import "sync/atomic"

type routerRegistrySnapshot struct {
	providers    map[PortName]Provider
	restrictions map[PortName][]string
}

var registry atomic.Pointer[routerRegistrySnapshot]

// RouterValidatePortName reports whether the port is declared in the router whitelist.
func RouterValidatePortName(port PortName) bool {
	switch port {
	case PortPrimary:
		return true
	case PortSecondary:
		return true
	case PortTertiary:
		return true
	case PortOptional:
		return true
	case PortCLIStyle:
		return true
	case PortCLIChrome:
		return true
	case PortCLIInteraction:
		return true
	case PortConfig:
		return true
	case PortWalk:
		return true
	case PortScanner:
		return true
	case PortReadFile:
		return true
	case PortCLIWrapperCore:
		return true
	case PortCLIWrapperDispatcher:
		return true
	case PortCLIWrapperSecurityGate:
		return true
	case PortCLIWrapperMacroRunner:
		return true
	case PortCLIWrapperFormatter:
		return true
	default:
		return false
	}
}
