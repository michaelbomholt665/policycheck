// internal/router/test_reset.go
// Provides test-only utilities for resetting the global router state
// across isolated execution environments and test permutations.

package router

// RouterResetForTest resets package-level router state for tests.
func RouterResetForTest() {
	registry.Store(nil)
}
