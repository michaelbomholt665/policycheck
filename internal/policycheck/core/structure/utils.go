// internal/policycheck/core/structure/utils.go
// Package structure provides utility functions for the structure package.
// It aids in determining if path rules apply.
package structure

import (
	"policycheck/internal/policycheck/host"
)

// HasPrefix returns true if the path has any of the given prefixes.
func HasPrefix(path string, prefixes []string) bool {
	return host.HasPrefix(path, prefixes)
}
