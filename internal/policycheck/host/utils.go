// internal/policycheck/host/utils.go
package host

import (
	"os"
	"path/filepath"

	"policycheck/internal/policycheck/utils"
)

// RelOrAbs returns the relative path of path from root, or path if it is already absolute.
func RelOrAbs(root, path string) (string, error) {
	if !filepath.IsAbs(path) {
		return utils.NormalizePolicyPath(path), nil
	}

	return utils.ToSlashRel(root, path), nil
}

// ReadFile reads the named file and returns the contents via the readfile provider.
func ReadFile(name string) ([]byte, error) {
	provider, err := ResolveReadFileProvider()
	if err != nil {
		// Fallback for bootstrap/config load if router not fully booted
		return os.ReadFile(name)
	}
	return provider.ReadFile(name)
}

// HasPrefix returns true if the string starts with any of the provided prefixes.
func HasPrefix(value string, prefixes []string) bool {
	return utils.HasPrefix(value, prefixes)
}

// NormalizePolicyPath normalizes a policy path for comparison.
func NormalizePolicyPath(value string) string {
	return utils.NormalizePolicyPath(value)
}
