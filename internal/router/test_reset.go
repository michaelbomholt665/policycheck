package router

// RouterResetForTest resets package-level router state for tests.
func RouterResetForTest() {
	registry.Store(nil)
}
