# CLI Wrapper TDD 7

## Objective

Finish convergence and hardening work so the implementation is structurally clean, design-aligned, and suitable for linking from the router repository without visible drift.

## Scope

- Remove duplicated wrapper-core logic and stale intermediate shapes.
- Finalize missing output and interaction behaviour from the design doc.
- Add targeted integration and hygiene checks that lock the final architecture in place.

## Task Checklist

- [ ] Remove duplicated wrapper core definitions from adapter packages.
- [ ] Make `internal/cliwrapper` the single authoritative wrapper core.
- [ ] Align remaining user-facing output with the design doc.
- [ ] Decide and document any intentionally reduced scope.
- [ ] Add structural tests or policy checks for shared entrypoint isolation.
- [ ] Add structural tests or policy checks for config filename stability.
- [ ] Reconcile the final implementation with the design doc.

## Why This Phase Exists

The current repo still contains duplicated wrapper logic and design drift, especially between:

- `internal/cliwrapper/*`
- `internal/adapters/cliwrapper/core.go`
- adapter-local config/helpers

Even after the feature surface is completed, the implementation should converge on one authoritative core.

## Dependencies

- `docs/command/cli-wrapper-TDD-5.md`
- `docs/command/cli-wrapper-TDD-6.md`

## File Plan

| File | Action | Purpose |
| --- | --- | --- |
| `internal/adapters/cliwrapper/core.go` | delete or collapse | Remove duplicate wrapper core definitions |
| `internal/adapters/cliwrapper/*.go` | update | Depend on `internal/cliwrapper` core instead of local copies |
| `internal/cliwrapper/*.go` | update | Become the single authoritative wrapper core |
| `internal/tests/cliwrapper/integration/...` | update | Final subsystem integration coverage |
| `internal/tests/policycheck/...` | update | Guard shared-binary coexistence if needed |
| `docs/command/policycheck-cli-wrapper-design.md` | update | Reconcile any deliberate post-implementation decisions |

## TDD Cycles

### T1 Core Convergence [ ]

Summary: remove duplicate wrapper-core types and logic so the package layout matches the documented architecture.

RED:
- [ ] Write a failing test or structural check that catches duplicated core definitions and adapter-local shadow types.
- [ ] Write a failing integration test proving adapters use the shared core package behaviour.

GREEN:
- [ ] Delete or collapse duplicated logic in `internal/adapters/cliwrapper/core.go`.
- [ ] Update adapters to depend on `internal/cliwrapper` as the only core wrapper package.

REFACTOR:
- [ ] Normalize names and comments so ownership is obvious at a glance.

Acceptance checks:
- There is one authoritative wrapper core.
- Adapters no longer shadow core behaviour.

### T2 Output and Interaction Design Alignment [ ]

Summary: implement or explicitly scope down any remaining user-facing behaviour from the design doc.

RED:
- [ ] Add a focused failing test for clearer block reason output.
- [ ] Add a focused failing test for consistent dry-run output.
- [ ] Add a focused failing test for optional prompt handling for moderate-risk flows if still in scope.

GREEN:
- [ ] Implement the missing output/interaction seams or update the design doc where scope was intentionally reduced.

REFACTOR:
- [ ] Keep rendering concerns out of low-level parser and adapter logic.

Acceptance checks:
- Final user-visible behaviour is coherent and documented.

### T3 Architecture Lock-In [ ]

Summary: protect the final shape with narrow, durable verification instead of broad brittle suites.

RED:
- [ ] Add a failing test or policy check for shared entrypoint isolation in `internal/app`.
- [ ] Add a failing test or policy check for no direct adapter-to-adapter imports.
- [ ] Add a failing test or policy check for wrapper surface reachability from the binary.
- [ ] Add a failing test or policy check for repo config filename stability.

GREEN:
- [ ] Implement any remaining cleanup exposed by those checks.

REFACTOR:
- [ ] Prefer structural tests and policy checks over giant end-to-end scripts.

Acceptance checks:
- The app is safe to link from the router repo without architectural embarrassment.

## Verification

- `go test ./internal/tests/cliwrapper/... -count=1`
- `go test ./internal/tests/policycheck/... -count=1`
- `go run ./cmd/policycheck --policy-list`
- `go run ./cmd/policycheck fmt headers --dry-run`
- `go run ./internal/router/tools/wrlk guide current`

## Exit Criteria

- Duplicated wrapper core logic is removed.
- Final behaviour matches either the design doc or an explicitly updated design.
- Structural drift is locked down with durable tests/policies.
