# Router usage rules for this repo:

- Treat the router as manifest-backed. Do not hand-edit generated router wiring unless explicitly required for consistency.
- Source of truth:
  - `internal/router/router_manifest.go` for router-owned ports and router-owned optional extensions
  - `internal/router/ext/app_manifest.go` for app-owned required adapters
- Generated runtime files:
  - `internal/router/ports.go`
  - `internal/router/registry_imports.go`
  - `internal/router/ext/optional_extensions.go`
  - `internal/router/ext/extensions.go`

Use `wrlk register` as the default mutation path:
- Add port: `go run ./internal/router/tools/wrlk register --port --router --name <PortName> --value <port-value>`
- Add router-owned extension: `go run ./internal/router/tools/wrlk register --ext --router --name <name>`
- Add app-owned adapter: `go run ./internal/router/tools/wrlk register --ext --app --name <name>`

Critical distinction:
- `register --ext --router` is only for router extensions under `internal/router/ext/extensions/<name>/`
- `register --ext --app` is only for app adapters outside the router, typically `internal/adapters/<name>/`
- Router-owned extensions always boot first
- App-owned adapters boot second, then their internal order is resolved from declared `Consumes()` dependencies

Do not:
- Reintroduce adapter imports into router core business logic
- Treat `extensions.go` as app-owned handwritten wiring; it is generated from `app_manifest.go`
- Use legacy commands (`wrlk add`, `wrlk ext add`, `wrlk ext install`, `wrlk ext app add`) except when documenting migration history

When changing router docs or code:
- Preserve the `register` workflow
- Keep manifests as the edit surface and generated files as runtime output
- Verify with `go test ./internal/tests/router/...`
- Run `go run ./internal/router/tools/wrlk --help` or another `wrlk` check before finishing