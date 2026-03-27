// internal/router/registry.go
// Provides functions to resolve extension providers from the core registry
// and enforces consumer access controls.

package router

// RouterResolveProvider resolves a provider from the published registry.
func RouterResolveProvider(port PortName) (Provider, error) {
	published := registry.Load()
	if published == nil {
		return nil, &RouterError{Code: RegistryNotBooted}
	}

	provider, exists := published.providers[port]
	if !exists {
		return nil, &RouterError{Code: PortNotFound, Port: port}
	}

	return provider, nil
}

// RouterResolveRestrictedPort resolves a provider and enforces consumer access control.
func RouterResolveRestrictedPort(port PortName, consumerID string) (Provider, error) {
	if consumerID == "" {
		return nil, &RouterError{Code: PortAccessDenied, Port: port, ConsumerID: consumerID}
	}

	provider, err := RouterResolveProvider(port)
	if err != nil {
		return nil, err
	}

	if !RouterCheckPortConsumerAccess(port, consumerID) {
		return nil, &RouterError{Code: PortAccessDenied, Port: port, ConsumerID: consumerID}
	}

	return provider, nil
}

// RouterCheckPortConsumerAccess is an internal helper to check access.
func RouterCheckPortConsumerAccess(port PortName, consumerID string) bool {
	published := registry.Load()
	if published == nil {
		return false
	}

	allowed, isRestricted := published.restrictions[port]
	if !isRestricted {
		// If no restriction is registered for this port, anyone can access it.
		return true
	}

	for _, id := range allowed {
		if id == "Any" || id == consumerID {
			return true
		}
	}

	return false
}
