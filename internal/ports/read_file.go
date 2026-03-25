// internal/ports/read_file.go
package ports

// ReadFileProvider defines the contract for reading file content.
type ReadFileProvider interface {
	// ReadFile reads the named file and returns the contents.
	ReadFile(name string) ([]byte, error)
}
