# CLI Wrapper TDD 4

## Objective

Finish the wrapper feature set with macros, `fmt headers`, router hardening, and integration verification. This phase should leave the CLI wrapper operational as its own subsystem, still sharing the app binary but not the policycheck domain model.

## Scope

- Implement `run` macros.
- Implement `fmt headers`.
- Finalize router-backed adapter wiring for the real wrapper adapters.
- Add integration and regression coverage around boundary rules.

## Testing Posture For This Phase

- [ ] Continue using TDD for the next behaviour slice only.
- [ ] Keep integration coverage narrow and only where it is needed to prove router wiring or subsystem separation.
- [ ] Do not chase coverage percentages.
- [ ] Defer broad post-implementation hardening suites until the wrapper functionality and shape stop moving.

## Dependencies

- `docs/command/cli-wrapper-TDD-1.md`
- `docs/command/cli-wrapper-TDD-2.md`
- `docs/command/cli-wrapper-TDD-3.md`
- `docs/router/cli-tools.md`

## File Plan

| File | Action | Purpose |
| --- | --- | --- |
| `internal/cliwrapper/template.go` | new | Macro template interpolation |
| `internal/cliwrapper/macro_runner.go` | new | Macro execution core |
| `internal/cliwrapper/header.go` | new | Header detection and injection |
| `internal/cliwrapper/walker.go` | new | Repo file discovery and skip logic |
| `internal/adapters/cliwrapper/macro_runner.go` | new/update | Router-resolved macro adapter |
| `internal/adapters/cliwrapper/format_headers.go` | new/update | Router-resolved fmt adapter |
| `internal/tests/cliwrapper/macro/macro_runner_test.go` | new | Macro tests |
| `internal/tests/cliwrapper/fmt/header_test.go` | new | Header logic tests |
| `internal/tests/cliwrapper/fmt/walker_test.go` | new | Walker tests |
| `internal/tests/cliwrapper/integration/` | new | Router and wrapper integration tests |

## Sequence

```mermaid
sequenceDiagram
    participant CLI as Wrapper Dispatcher
    participant Macro as Macro Runner
    participant Exec as Command Executor
    participant Fmt as Header Formatter
    participant FS as Repository Files
    CLI->>Macro: run named macro
    Macro->>Exec: execute step sequence
    Exec-->>Macro: step result
    CLI->>Fmt: run headers command
    Fmt->>FS: scan repo files
    FS-->>Fmt: target file list
    Fmt->>FS: inject or verify headers
```

## Component Sketch

```plantuml
@startuml
package "CLI Wrapper Core" {
  class MacroTemplate
  class MacroRunner
  class HeaderFormatter
  class HeaderWalker
}

package "Adapter Boundary" {
  interface CLIWrapperMacroRunner
  interface CLIWrapperFormatter
}

MacroRunner ..> MacroTemplate
HeaderFormatter ..> HeaderWalker
CLIWrapperMacroRunner <|.. MacroRunner
CLIWrapperFormatter <|.. HeaderFormatter
@enduml
```

## TDD Cycles

### T1 Macro Templates and Execution [ ]

Summary: implement named wrapper macros without letting them drift into generic app orchestration.

RED:
- [ ] Write the smallest set of failing tests needed to establish macro execution, stop-on-failure, and template substitution.
- [ ] Add config-resolution tests only when the macro path depends on them directly.

GREEN:
- [ ] Implement `template.go` and `macro_runner.go`.
- [ ] Support prompt-free variable injection from provided arguments first; interactive prompting can remain a later enhancement if not required immediately.
- [ ] Return aggregate failure context when `on_failure = "continue"` still encounters errors.

REFACTOR:
- [ ] Share subprocess cleanup helpers with tooling/package execution if the contract is already stable.
- [ ] Keep macro parsing separate from command execution.

Best practices and standards:
- [ ] Every failing step must surface its command text.
- [ ] Child process cleanup remains mandatory.
- [ ] Avoid hidden mutation of wrapper config during runtime.
- [ ] Keep macro tests focused on the next behaviour, not the eventual full matrix.

Acceptance checks:
- [ ] Macro runner tests pass.
- [ ] Macro execution stays inside the wrapper subsystem.

### T2 `fmt headers` Core Logic [ ]

Summary: implement idempotent repository header maintenance for Go, Python, and TypeScript.

RED:
- [ ] Write only the minimal failing tests needed to establish one header path per supported language and one idempotence check.
- [ ] Expand skip-directory and dry-run cases only as the implementation reaches them.

GREEN:
- [ ] Implement `header.go` and `walker.go`.
- [ ] Detect stale or missing headers and inject the correct repo-relative path comment.
- [ ] Support dry-run reporting without file writes.

REFACTOR:
- [ ] Extract language-specific header rules into small helpers if the file becomes branch-heavy.
- [ ] Keep filesystem traversal separate from content mutation.

Best practices and standards:
- [ ] Never modify skipped directories.
- [ ] Preserve Python shebangs.
- [ ] Make dry-run output deterministic for test assertions.
- [ ] Avoid front-loading every filesystem case before the formatter shape is stable.

Acceptance checks:
- [ ] Header and walker tests pass.
- [ ] A second write run reports zero modifications.

### T3 Router Wiring for Real Adapters [ ]

Summary: replace placeholder adapter registrations with the real wrapper adapters using the approved router tooling workflow.

RED:
- [ ] Add router integration tests that fail until the dispatcher, security, macro, and fmt ports resolve to real adapters.
- [ ] Add a regression test proving no adapter imports another adapter to obtain dependencies.

GREEN:
- [ ] Run the documented `wrlk add` commands for any remaining ports.
- [ ] Wire real application adapters through the mutable router extension path, following `docs/router/cli-tools.md`.
- [ ] Verify the router inventory with `go run ./internal/router/tools/wrlk guide current`.

REFACTOR:
- [ ] Remove stale placeholder registrations and dead code once the real adapters are wired.
- [ ] Tighten package docs so ownership of router-wired adapters is obvious.

Best practices and standards:
- [ ] Do not edit frozen router files by hand.
- [ ] Stop if router lock output drifts.
- [ ] Keep adapter imports restricted to ports and router seams.
- [ ] Keep router integration tests to a minimum required proof set.

Acceptance checks:
- [ ] Router integration tests pass.
- [ ] `guide current` shows the intended wrapper capabilities.

### T4 Integration and Completion Gate [ ]

Summary: prove the wrapper works as a coherent subsystem and does not regress policycheck.

RED:
- [ ] Add only the minimal final proof tests needed to show wrapper/policycheck separation and one end-to-end success path per implemented feature.
- [ ] Leave broader regression expansion for a later hardening phase if real defects justify it.

GREEN:
- [ ] Implement any missing seams exposed by the integration failures.
- [ ] Add concise user-facing output formatting for wrapper actions and block reasons.
- [ ] Document the final command surface in the docs if implementation details changed from the original design.

REFACTOR:
- [ ] Remove duplicated execution helpers across wrapper features.
- [ ] Keep integration fixtures small and explicit.

Best practices and standards:
- [ ] Prefer targeted integration tests over broad fragile end-to-end scripts.
- [ ] Keep wrapper logs readable but deterministic.
- [ ] Do not broaden the scope into policycheck analysis internals.
- [ ] Treat this as a thin completion gate, not a coverage sweep.

Acceptance checks:
- [ ] Integration tests pass.
- [ ] Wrapper and policycheck paths coexist without domain leakage.

## Verification

- [ ] `go test ./internal/tests/cliwrapper/macro/... -count=1`
- [ ] `go test ./internal/tests/cliwrapper/fmt/... -count=1`
- [ ] `go test ./internal/tests/cliwrapper/integration/... -count=1`
- [ ] `go run ./internal/router/tools/wrlk guide current`
- [ ] `go run ./cmd/policycheck`

## Exit Criteria

- [ ] Macros work.
- [ ] `fmt headers` works.
- [ ] Real adapters are router-wired.
- [ ] Integration coverage protects the wrapper/policycheck boundary.
