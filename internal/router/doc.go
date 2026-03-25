// Package router Provides the zero-dependency port registry and extension boot machinery.
//
// Package Concerns:
// - This package must never import any internal package; ext handles all adapter wiring.
// - Callers boot via ext.RouterBootExtensions and resolve via RouterResolveProvider.
//
// # AI AND AGENT IMPLEMENTATION PROTOCOL (STRICT)
//
// ### 1. THE GOLDEN RULE: ARCHITECTURAL BLINDNESS
// The Router CORE (extension.go, registry.go, error_surface.go) is a FROZEN KERNEL.
// - DO NOT: Add imports to any package inside internal/* to the core.
// - DO NOT: Add business logic, domain types, or utility functions to the core.
// - DO NOT: Attempt to bypass RouterResolveProvider by using global variables or init() functions.
//
// ### 2. THE LOCK PROTOCOL (ANTI-TAMPERING)
// This package is protected by 'router.lock'. Manual edits to the Core will break the checksums
// and fail the CI/CD pipeline.
// - AGENT ACTION: If you need to add a Port, you MUST NOT manually edit ports.go.
// - AGENT ACTION: Use the command: `go run ./internal/router/tools/wrlk add --name <PortName> --value <string>`
// - RESULT: The tool will update ports.go, registry_imports.go, and re-calculate the lock.
//
// ### 3. FORBIDDEN PATTERNS (AI CHEAT-SHEET)
//   - NO SHARED STATE: Extensions must not share state via global variables. All sharing happens through Ports.
//   - NO IMPORTS IN ADAPTERS: Adapters MUST NOT import other adapters. They only import 'router' and 'ports'.
//   - ANY BOUNDARY RULE: 'any' is allowed only in contract-blind infrastructure such as the registry,
//     tooling, and opaque relays. Business logic and consumers must cast to the Port interface before use.
//   - NO MANUAL BOOTING: Only ext.RouterBootExtensions is authorized to trigger the loading sequence.
//
// ### 4. EXTENSION IMPLEMENTATION STEPS
// 1. SCAFFOLD: Run `wrlk add` to register the PortName.
// 2. DEFINE: Create the interface in the domain-specific package (internal/ports).
// 3. IMPLEMENT: Create an Extension struct in internal/ext.
// 4. REGISTER: Use reg.RouterRegisterProvider(router.PortName, implementation).
// 5. WIRE:
//   - Optional capability extension: `go run ./internal/router/tools/wrlk ext add --name <ExtensionName>`
//   - Required application extension: `go run ./internal/router/tools/wrlk ext app add --name <ExtensionName>`
//
// ### 5. PERFORMANCE INVARIANT
// This router is optimized for <1ns resolution.
// - DO NOT: Add Mutexes, RWMutexes, or complex locking to the resolution path.
// - DO NOT: Add logging or telemetry inside RouterResolveProvider.
// - REQUIRED: All resolution must remain O(1) via atomic.Pointer snapshots.
package router
