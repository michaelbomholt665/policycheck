package router

import "fmt"

const dependencyOrderViolationGuidance = "If this port is registered in extensions.go or optional_extensions.go, the initialization order is wrong. Move the providing extension higher up in the correct extensions slice."

type routerErrorDescriptor struct {
	render func(err *RouterError) string
}

var routerErrorCatalog = map[RouterErrorCode]routerErrorDescriptor{
	PortUnknown: {
		render: func(err *RouterError) string {
			return fmt.Sprintf("port %q is not a declared port", err.Port)
		},
	},
	PortDuplicate: {
		render: func(err *RouterError) string {
			return fmt.Sprintf("port %q already registered", err.Port)
		},
	},
	InvalidProvider: {
		render: func(err *RouterError) string {
			return "provider is invalid"
		},
	},
	PortNotFound: {
		render: func(err *RouterError) string {
			return fmt.Sprintf("port %q not found", err.Port)
		},
	},
	RegistryNotBooted: {
		render: func(err *RouterError) string {
			return "router registry not booted"
		},
	},
	PortContractMismatch: {
		render: func(err *RouterError) string {
			return fmt.Sprintf("provider for port %q does not satisfy the expected contract", err.Port)
		},
	},
	RequiredExtensionFailed: {
		render: func(err *RouterError) string {
			return renderRouterErrorCause(err, "required extension failed")
		},
	},
	OptionalExtensionFailed: {
		render: func(err *RouterError) string {
			return renderRouterErrorCause(err, "optional extension failed")
		},
	},
	DependencyOrderViolation: {
		render: func(err *RouterError) string {
			return fmt.Sprintf("port %q dependency order violation. %s", err.Port, dependencyOrderViolationGuidance)
		},
	},
	AsyncInitTimeout: {
		render: func(err *RouterError) string {
			return renderRouterErrorCause(err, "async extension initialization timed out")
		},
	},
	MultipleInitializations: {
		render: func(err *RouterError) string {
			return "router already initialized"
		},
	},
	PortAccessDenied: {
		render: func(err *RouterError) string {
			return fmt.Sprintf("consumer %q access denied to restricted port %q", err.ConsumerID, err.Port)
		},
	},
	RouterProfileInvalid: {
		render: func(err *RouterError) string {
			return renderRouterErrorCause(err, "router profile is invalid")
		},
	},
	RouterEnvironmentMismatch: {
		render: func(err *RouterError) string {
			return renderRouterErrorCause(err, "router profile does not match the runtime environment")
		},
	},
}

var routerErrorRenderer = defaultRouterErrorRenderer

// renderRouterError renders a router error through the active internal renderer seam.
func renderRouterError(err *RouterError) string {
	return routerErrorRenderer(err)
}

// defaultRouterErrorRenderer renders router errors using the canonical router catalog.
func defaultRouterErrorRenderer(err *RouterError) string {
	if err == nil {
		return ""
	}

	descriptor, exists := routerErrorCatalog[err.Code]
	if !exists || descriptor.render == nil {
		if err.Err != nil {
			return err.Err.Error()
		}

		return string(err.Code)
	}

	return descriptor.render(err)
}

// renderRouterErrorCause appends an underlying cause to a fallback router message.
func renderRouterErrorCause(err *RouterError, fallback string) string {
	if err == nil {
		return fallback
	}

	if err.Err != nil {
		return fmt.Sprintf("%s: %s", fallback, err.Err)
	}

	return fallback
}
