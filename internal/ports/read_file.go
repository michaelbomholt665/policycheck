// internal/ports/read_file.go
// Declares the router port for file-content reads used by policy checks.
// Keeps file access behind a small contract that tests can substitute.
package ports

// ReadFileProvider defines the contract for reading file content.
type ReadFileProvider interface {
	// ReadFile reads the named file and returns the contents.
	ReadFile(name string) ([]byte, error)
}
