# CLI Wrapper TDD 2

## Objective

Build the wrapper control plane: config resolution, wrapper command classification, and router-resolved orchestration. This phase should still avoid real package installation or macro execution. The goal is to prove that wrapper requests flow through their own config and dispatch pipeline.

## Scope

- Add wrapper-specific config shapes and loader behaviour.
- Resolve repo and global config for wrapper use.
- Classify incoming CLI input into wrapper modes.
- Keep policycheck analysis entrypoints untouched unless they are explicitly delegated to from the shared top-level app.

## Testing Posture For This Phase

- [ ] Use tests only to drive the next config or dispatch design decision.
- [ ] Avoid building broad regression suites around config and bootstrap code that is still likely to be simplified.
- [ ] Prefer one focused RED test per behaviour slice over many anticipatory cases.
- [ ] Defer fuller regression and integration coverage until the wrapper control plane stabilizes.

## Dependencies

- `docs/command/cli-wrapper-TDD-1.md`
- `docs/router/cli-tools.md`

## File Plan

| File | Action | Purpose |
| --- | --- | --- |
| `internal/cliwrapper/doc.go` | new | Wrapper application-layer docs |
| `internal/cliwrapper/config.go` | new | Wrapper-only config structs and validation |
| `internal/cliwrapper/config_loader.go` | new | Global + repo config loading for wrapper |
| `internal/cliwrapper/detector.go` | new | Classify passthrough, package gate, tooling gate, macro, fmt |
| `internal/tests/cliwrapper/config/config_loader_test.go` | new | RED/GREEN config tests |
| `internal/tests/cliwrapper/detector/detector_test.go` | new | RED/GREEN mode detection tests |
| `internal/tests/cliwrapper/boot/boot_test.go` | new | Router-resolved wrapper boot tests |

## Sequence

```mermaid
sequenceDiagram
    participant CLI as Shared App Entry
    participant Detect as Wrapper Detector
    participant Load as Wrapper Config Loader
    participant Router as Router
    participant Port as Wrapper Dispatcher Port
    CLI->>Detect: classify args
    Detect-->>CLI: wrapper mode
    CLI->>Load: load global + repo wrapper config
    Load-->>CLI: validated wrapper config
    CLI->>Router: resolve wrapper ports
    Router-->>CLI: dispatcher placeholder
    CLI->>Port: dispatch classified request
```

## Component Sketch

```plantuml
@startuml
package "Shared App" {
  class RunEntry
}

package "CLI Wrapper Core" {
  class WrapperConfigLoader
  class WrapperDetector
  class WrapperBootstrap
}

package "Router Boundary" {
  interface CLIWrapperDispatcher
}

RunEntry --> WrapperDetector
RunEntry --> WrapperConfigLoader
RunEntry --> CLIWrapperDispatcher
@enduml
```

## TDD Cycles

### T1 Wrapper Config Schema [ ]

Summary: define a wrapper-local schema so wrapper policies do not piggyback on policycheck config semantics by accident.

RED:
- [ ] Write tests that fail until wrapper config structs support `security`, `tooling.gates`, `macros`, and `ui`.
- [ ] Add validation tests for repo config trying to become less strict than global config.

GREEN:
- [ ] Implement wrapper config structs under `internal/cliwrapper/config.go`.
- [ ] Support wrapper-local validation helpers for severity ordering and macro shape.
- [ ] Keep the schema separate from existing policycheck analysis config types unless reuse is deliberate and documented.

REFACTOR:
- [ ] Normalize severity helpers into small reusable functions.
- [ ] Remove duplicated validation logic from tests once behaviour is stable.

Best practices and standards:
- [ ] Add doc comments for exported types.
- [ ] Wrap parse and validation errors with file-scope context.
- [ ] Keep config structs intentionally narrow for this subsystem.

Acceptance checks:
- [ ] Tests prove the wrapper config can evolve independently.
- [ ] The phase does not require real command execution yet.

### T2 Wrapper Config Loader [ ]

Summary: load global and repo config for the wrapper using upward repo-root resolution and documented merge rules.

RED:
- [ ] Write the minimum set of failing tests needed to define global-only load, repo override, and invalid threshold relaxation.
- [ ] Add only the essential fallback test for missing repo config.

GREEN:
- [ ] Implement `config_loader.go` with explicit load order.
- [ ] Walk upward from the current working directory to locate `policy-gate.toml`.
- [ ] Merge repo config over global config while enforcing the stricter-only security rule.

REFACTOR:
- [ ] Split file lookup from merge logic if the loader becomes complex.
- [ ] Keep cognitive complexity within repository limits.

Best practices and standards:
- [ ] No singleton config cache.
- [ ] Fresh config load per command.
- [ ] Return actionable errors that identify whether global or repo config failed.
- [ ] Do not expand the test matrix beyond what the current implementation step needs.

Acceptance checks:
- [ ] Loader tests pass.
- [ ] Missing repo config falls back to global-only behaviour.

### T3 Wrapper Mode Detection [ ]

Summary: classify incoming args into wrapper modes before any adapter executes.

RED:
- [ ] Write only the focused failing tests needed to distinguish `run`, `fmt headers`, package installs, `-then`, and passthrough.
- [ ] Keep the `go test` passthrough case as a single explicit regression guard.

GREEN:
- [ ] Implement `detector.go`.
- [ ] Return a small enum or typed mode for `Passthrough`, `PackageGate`, `ToolingGate`, `MacroRun`, and `FormatHeaders`.
- [ ] Keep the detection rules deterministic and easy to extend.

REFACTOR:
- [ ] Extract manager and subcommand tables if hard-coded branching becomes noisy.
- [ ] Remove duplicated normalization logic across tests and implementation.

Best practices and standards:
- [ ] Prefer table-driven tests.
- [ ] Do not infer wrapper intent from shell syntax the process never receives.
- [ ] Treat unknown commands as passthrough by default.
- [ ] Keep tables small while the classification rules are still evolving.

Acceptance checks:
- [ ] The detector can route wrapper features without touching real execution logic.
- [ ] Tests document the subsystem boundary clearly.

### T4 Router-Resolved Wrapper Bootstrap [ ]

Summary: prove the shared app boot can classify wrapper work, load wrapper config, resolve wrapper ports, and hand off to the wrapper subsystem.

RED:
- [ ] Write a boot test that expects the wrapper dispatcher port to be resolved and invoked when wrapper mode is selected.
- [ ] Write a separate test that expects normal policycheck execution to remain unaffected for policycheck-specific commands.

GREEN:
- [ ] Add the bootstrap seam in the shared app layer.
- [ ] Resolve wrapper dependencies through the router boundary only.
- [ ] Pass a wrapper request object that contains the classified mode, raw args, cwd, and loaded config.

REFACTOR:
- [ ] Tighten request and response types so later phases can add execution details without widening every call signature.

Best practices and standards:
- [ ] Shared entrypoint, separate request model.
- [ ] No direct adapter imports from the shared boot path.
- [ ] Boot tests should focus on selection logic, not adapter internals.
- [ ] Resist adding extra boot permutations until the handoff seam has settled.

Acceptance checks:
- [ ] Wrapper bootstrap tests pass.
- [ ] Policycheck-only commands still follow their existing path.

## Verification

- [ ] `go test ./internal/tests/cliwrapper/config/... -count=1`
- [ ] `go test ./internal/tests/cliwrapper/detector/... -count=1`
- [ ] `go test ./internal/tests/cliwrapper/boot/... -count=1`
- [ ] `go run ./cmd/policycheck`

Verification note: stop after the targeted TDD cycle passes; do not inflate this phase with coverage-oriented follow-up tests.

## Exit Criteria

- [ ] Wrapper config loads independently.
- [ ] Wrapper mode detection is stable.
- [ ] Shared app boot can hand off to wrapper placeholders through the router.
