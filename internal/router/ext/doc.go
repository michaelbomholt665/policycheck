// Package ext wires the concrete provider extensions into the router.
//
// Package Concerns:
//   - Only package permitted to import both internal/router and internal/adapters.
//   - All application boot entry points must call ext.RouterBootExtensions, which
//     keeps extension activation explicit and compile-time.
//   - `wrlk register --ext --router` records router-owned optional capability
//     extensions in router_manifest.go and regenerates optional_extensions.go.
//   - `wrlk register --ext --app` records required application adapters in
//     app_manifest.go and regenerates extensions.go.
package ext
