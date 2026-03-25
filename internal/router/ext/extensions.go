package ext

import (
	"context"
	"fmt"
	"os"
	"strings"

	"policycheck/internal/adapters/config"
	"policycheck/internal/adapters/scanners"
	"policycheck/internal/adapters/walk"
	"policycheck/internal/router"
)

// extensions contains required application extensions only.
// Keep this slice explicit and app-owned; do not leave sample providers wired here.
var extensions = []router.Extension{
	&config.Extension{},
	&walk.Extension{},
	&scanners.Extension{},
}

const (
	wrlkEnvKey        = "WRLK_ENV"
	routerProfileKey  = "ROUTER_PROFILE"
	routerAllowAnyKey = "ROUTER_ALLOW_ANY"
)

// RouterBuildExtensionBundle returns the compiled-in optional and application extension bundles.
func RouterBuildExtensionBundle() ([]router.Extension, []router.Extension) {
	return append([]router.Extension(nil), optionalExtensions...), append([]router.Extension(nil), extensions...)
}

// RouterBootExtensions wires optional extensions first, then application extensions.
func RouterBootExtensions(ctx context.Context) ([]error, error) {
	if err := validateRouterBootPolicy(); err != nil {
		return nil, err
	}

	optionalBundle, applicationBundle := RouterBuildExtensionBundle()
	return router.RouterLoadExtensions(optionalBundle, applicationBundle, ctx)
}

// validateRouterBootPolicy rejects router profile combinations that are unsafe at boot.
func validateRouterBootPolicy() error {
	runtimeEnv := normalizeRouterProfile(os.Getenv(wrlkEnvKey))
	declaredProfile := normalizeRouterProfile(os.Getenv(routerProfileKey))

	if declaredProfile != "" && runtimeEnv != "" && declaredProfile != runtimeEnv {
		return &router.RouterError{
			Code: router.RouterEnvironmentMismatch,
			Err: fmt.Errorf(
				"%s=%q does not match %s=%q",
				routerProfileKey,
				declaredProfile,
				wrlkEnvKey,
				runtimeEnv,
			),
		}
	}

	if runtimeEnv == "prod" && parseRouterBoolEnv(os.Getenv(routerAllowAnyKey)) {
		return &router.RouterError{
			Code: router.RouterProfileInvalid,
			Err: fmt.Errorf(
				"%s=true is not allowed when %s=%q",
				routerAllowAnyKey,
				wrlkEnvKey,
				runtimeEnv,
			),
		}
	}

	return nil
}

// normalizeRouterProfile trims and lowercases a router profile value for stable comparison.
func normalizeRouterProfile(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// parseRouterBoolEnv reports whether an environment value should be treated as enabled.
func parseRouterBoolEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	default:
		return false
	}
}
