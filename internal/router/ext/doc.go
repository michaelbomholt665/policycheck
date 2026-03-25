// Package ext Wires the concrete provider extensions into the router.
//
// Package Concerns:
//   - Only package permitted to import both internal/router and internal/adapters.
//   - All application boot entry points must call ext.RouterBootExtensions, which
//     keeps extension activation explicit and compile-time.
//   - `wrlk ext add` scaffolds optional capability extensions and wires
//     optional_extensions.go.
//   - `wrlk ext app add` scaffolds required application extensions and wires
//     extensions.go.
package ext
