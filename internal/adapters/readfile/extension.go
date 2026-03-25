// internal/adapters/readfile/extension.go
package readfile

import (
	"os"

	"policycheck/internal/router"
)

// Extension implements router.Extension for the readfile adapter.
type Extension struct{}

// Required returns true - readfile capability is mandatory.
func (e *Extension) Required() bool { return true }

// Consumes returns nil.
func (e *Extension) Consumes() []router.PortName { return nil }

// Provides returns the ports this extension registers.
func (e *Extension) Provides() []router.PortName { return []router.PortName{router.PortReadFile} }

// RouterProvideRegistration registers the readfile provider.
func (e *Extension) RouterProvideRegistration(reg *router.Registry) error {
	return reg.RouterRegisterProvider(router.PortReadFile, &Adapter{})
}

// ExtensionInstance returns the extension instance.
func ExtensionInstance() router.Extension {
	return &Extension{}
}

// Adapter implements the ports.ReadFileProvider interface.
type Adapter struct{}

// ReadFile reads the named file and returns the contents.
func (a *Adapter) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}
