// internal/adapters/config/extension.go
package config

import (
	"os"

	"policycheck/internal/router"
)

// Extension implements router.Extension for the config adapter.
type Extension struct{}

// Required returns true - config is mandatory for boot.
func (e *Extension) Required() bool {
	return true
}

// Consumes returns nil - no boot-time dependencies.
func (e *Extension) Consumes() []router.PortName {
	return nil
}

// Provides returns the ports this extension registers.
func (e *Extension) Provides() []router.PortName {
	return []router.PortName{router.PortConfig}
}

// RouterProvideRegistration registers the config provider.
func (e *Extension) RouterProvideRegistration(reg *router.Registry) error {
	return reg.RouterRegisterProvider(router.PortConfig, &Adapter{})
}

// ExtensionInstance returns the extension instance.
func ExtensionInstance() router.Extension {
	return &Extension{}
}

// Adapter implements the ports.ConfigProvider interface.
type Adapter struct {
	injectedPath string
}

// SetPath informs the provider of a specific config file location.
func (a *Adapter) SetPath(path string) {
	a.injectedPath = path
}

// GetRawSource returns the raw configuration bytes from a default location.
func (a *Adapter) GetRawSource() ([]byte, error) {
	path := "policy-gate.toml"
	if a.injectedPath != "" {
		path = a.injectedPath
	}
	return os.ReadFile(path)
}
