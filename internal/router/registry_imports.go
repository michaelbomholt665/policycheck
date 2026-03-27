package router

import "sync/atomic"

type routerSnapshot struct {
	providers    map[PortName]Provider
	restrictions map[PortName][]string
}

var registry atomic.Pointer[routerSnapshot]

// RouterValidatePortName reports whether the port is declared in the router whitelist.
func RouterValidatePortName(port PortName) bool {
	switch port {
	case PortPrimary, PortSecondary, PortTertiary, PortOptional, PortCLIStyle, PortCLIChrome, PortCLIInteraction, PortConfig, PortWalk, PortScanner, PortReadFile, PortCLIWrapperDispatcher, PortCLIWrapperSecurityGate, PortCLIWrapperMacroRunner, PortCLIWrapperFormatter, PortCLIWrapperCore:
		return true
	default:
		return false
	}
}
