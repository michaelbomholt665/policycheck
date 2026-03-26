// internal/app/doc.go
// Package app Provides the application entry point and bootstrap logic.
// It coordinates the initialization of the router and core services, and
// provides the shared binary dispatch seam between the wrapper and analysis
// surfaces.
//
// Package Concerns:
// - Coordinates policycheck initialization, bootstrapping, and lifecycle.
// - Routes CLI commands to either the wrapper subsystem or the analysis engine.
package app
