# CLI Wrapper TDD 6

## Objective

Converge the wrapper config implementation onto the contract described in `policycheck-cli-wrapper-design.md`, removing the current drift between `wrapper-gate.toml`, `policy-gate.toml`, and the simplified wrapper schema.

## Scope

- Standardize repo config discovery on `policy-gate.toml`.
- Define one authoritative wrapper config schema and merge model.
- Implement the `policycheck config` inspection/init surface promised by the design doc.
- Add explicit risk-override handling such as `--allow-risk=<level>`.

## Task Checklist

- [ ] Replace `wrapper-gate.toml` repo discovery with `policy-gate.toml`.
- [ ] Remove duplicate config-path discovery logic across wrapper packages.
- [ ] Align wrapper config schema with the design doc.
- [ ] Implement documented global + repo merge semantics.
- [ ] Add `policycheck config` inspection commands.
- [ ] Add `policycheck config init` scaffold commands.
- [ ] Implement explicit `--allow-risk=<level>` handling in package-gate flow.
- [ ] Add focused tests for config commands and risk override behaviour.

## Why This Phase Exists

The design doc specifies one visible repo config file and a richer config model than the current implementation. The current code mixes:

- `wrapper-gate.toml` discovery in core wrapper loader
- `wrapper-gate.toml` plus `policy-gate.toml` discovery in adapter code
- a simpler `block_threshold` schema instead of the design doc's policy lists and config commands

This phase makes the config story coherent and user-facing.

## Dependencies

- `docs/command/policycheck-cli-wrapper-design.md`
- `docs/command/cli-wrapper-TDD-5.md`

## File Plan

| File | Action | Purpose |
| --- | --- | --- |
| `internal/cliwrapper/config.go` | update | Align schema with the design doc |
| `internal/cliwrapper/config_loader.go` | update | Use `policy-gate.toml` repo resolution and documented merge rules |
| `internal/adapters/cliwrapper/macro_runner.go` | update | Remove duplicate config resolution drift |
| `internal/adapters/cliwrapper/dispatcher.go` | update | Honor `--allow-risk=<level>` and config-backed security policy |
| `internal/app/run.go` | update | Add `policycheck config` subcommands |
| `internal/tests/cliwrapper/config/...` | update | TDD for merged config behaviour |
| `internal/tests/app/config/...` | new | TDD for config command surface |

## TDD Cycles

### T1 Config Filename and Resolution Convergence [ ]

Summary: make `policy-gate.toml` the visible repo config file for the shared binary as described in the design doc.

RED:
- [ ] Write a failing test that wrapper config is found via upward search for `policy-gate.toml`.
- [ ] Write a failing test that `wrapper-gate.toml` is not required for normal repo operation.

GREEN:
- [ ] Change repo config discovery to `policy-gate.toml`.
- [ ] Remove duplicate repo-root search logic that disagrees across packages.

REFACTOR:
- [ ] Centralize config-path resolution in one wrapper config path helper.

Acceptance checks:
- Wrapper config resolution matches the design doc.
- There is one authoritative repo config filename.

### T2 Config Schema and Merge Semantics [ ]

Summary: align the code with the design doc's global + repo layering model and stricter-only security rule.

RED:
- [ ] Add a failing test for global + repo merge by key.
- [ ] Add a failing test that repo cannot relax global blocking policy.
- [ ] Add a failing test for macro merge semantics.
- [ ] Add a failing test for tooling gate merge semantics.
- [ ] Add a failing test for unknown fields if the strictness contract requires it.

GREEN:
- [ ] Replace or extend `block_threshold` with the documented security policy shape.
- [ ] Implement the documented merge model for macros, tooling gates, and UI config.

REFACTOR:
- [ ] Remove drift between core wrapper config and adapter-local config clones.

Acceptance checks:
- Config matches the design doc instead of an intermediate schema.

### T3 Risk Override and Config Commands [ ]

Summary: implement the missing user-facing control plane described by the design doc.

RED:
- [ ] Write a failing test for `policycheck config`.
- [ ] Write a failing test for `policycheck config --global`.
- [ ] Write a failing test for `policycheck config init`.
- [ ] Write a failing test for `policycheck config init --global`.
- [ ] Write a failing test for `--allow-risk=<level>` blocking/override behaviour.

GREEN:
- [ ] Add config inspection and scaffold commands at the shared app layer.
- [ ] Implement explicit risk override parsing and enforcement in wrapper package-gate flow.

REFACTOR:
- [ ] Keep config rendering and config mutation logic separate from execution adapters.

Acceptance checks:
- The config UX promised by the design doc exists.
- Risk overrides are explicit and policy-aware.

## Verification

- `go test ./internal/tests/cliwrapper/config/... -count=1`
- `go test ./internal/tests/app/config/... -count=1`
- `go run ./cmd/policycheck config`
- `go run ./cmd/policycheck config init --dry-run`

## Exit Criteria

- Wrapper config discovery uses `policy-gate.toml`.
- One coherent config schema exists.
- Config commands and risk overrides are implemented.
