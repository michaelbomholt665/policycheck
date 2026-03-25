// internal/policycheck/host/ports.go
package host

import (
	"fmt"

	"policycheck/internal/ports"
	"policycheck/internal/router"
)

// ResolveConfigProvider resolves and casts the config provider.
func ResolveConfigProvider() (ports.ConfigProvider, error) {
	provider, err := router.RouterResolveProvider(router.PortConfig)
	if err != nil {
		return nil, fmt.Errorf("resolve config provider: %w", err)
	}

	configProvider, ok := provider.(ports.ConfigProvider)
	if !ok {
		return nil, &router.RouterError{
			Code: router.PortContractMismatch,
			Port: router.PortConfig,
		}
	}

	return configProvider, nil
}

// SetInjectedPath resolves the config provider and sets the injected path.
func SetInjectedPath(path string) error {
	provider, err := ResolveConfigProvider()
	if err != nil {
		return err
	}
	provider.SetPath(path)
	return nil
}

// ResolveWalkProvider resolves and casts the walk provider.
func ResolveWalkProvider() (ports.WalkProvider, error) {
	provider, err := router.RouterResolveProvider(router.PortWalk)
	if err != nil {
		return nil, fmt.Errorf("resolve walk provider: %w", err)
	}

	walkProvider, ok := provider.(ports.WalkProvider)
	if !ok {
		return nil, &router.RouterError{
			Code: router.PortContractMismatch,
			Port: router.PortWalk,
		}
	}

	return walkProvider, nil
}

// ResolveScannerProvider resolves and casts the scanner provider.
func ResolveScannerProvider() (ports.ScannerProvider, error) {
	provider, err := router.RouterResolveProvider(router.PortScanner)
	if err != nil {
		return nil, fmt.Errorf("resolve scanner provider: %w", err)
	}

	scannerProvider, ok := provider.(ports.ScannerProvider)
	if !ok {
		return nil, &router.RouterError{
			Code: router.PortContractMismatch,
			Port: router.PortScanner,
		}
	}

	return scannerProvider, nil
}

// ResolveReadFileProvider resolves and casts the readfile provider.
func ResolveReadFileProvider() (ports.ReadFileProvider, error) {
	provider, err := router.RouterResolveProvider(router.PortReadFile)
	if err != nil {
		return nil, fmt.Errorf("resolve readfile provider: %w", err)
	}

	readFileProvider, ok := provider.(ports.ReadFileProvider)
	if !ok {
		return nil, &router.RouterError{
			Code: router.PortContractMismatch,
			Port: router.PortReadFile,
		}
	}

	return readFileProvider, nil
}
