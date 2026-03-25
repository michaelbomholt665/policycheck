// internal/ports/config.go
package ports

// ConfigProvider defines the contract for providing raw configuration.
type ConfigProvider interface {
	// SetPath informs the provider of a specific config file location.
	SetPath(path string)

	// GetRawSource returns the raw configuration bytes.
	GetRawSource() ([]byte, error)
}
