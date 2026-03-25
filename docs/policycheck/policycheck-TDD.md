# policycheck Rewrite - TDD Strategy

**Version 1.0 · March 2026**

---

## 1. Goal

This document defines the test-driven development strategy for the `policycheck`
rewrite described in `docs/policycheck/policycheck-design.md`.

The strategy exists to keep the rewrite controlled, incremental, and provable.
The central rule is simple:

**Every structural change in the rewrite must be driven by a failing test at the
smallest useful layer first.**

That means:

- pure logic is driven by pure unit tests
- filesystem orchestration is driven by minimal integration tests
- host capability usage is driven by boundary tests
- hardcoded behavior removal is driven by config-override tests
- the final acceptance gate is that the binary passes against its own source

This is not optional process overhead. It is the mechanism that prevents the
rewrite from drifting away from the current behavior while packages, seams, and
providers are being restructured.

---

## 2. Core TDD Rules

### 2.1 Red-Green-Refactor at the Correct Layer

The rewrite has two implementation layers and they must be tested separately:

| Layer | What changes here | First test to write |
|---|---|---|
| Pure logic | threshold math, AST decisions, string matching, pattern filtering, config validation | unit test with inline inputs |
| Orchestration | file enumeration, config loading, scanner dispatch, router-backed providers | integration test with `t.TempDir()` or provider seam |

Do not mix both layers in a single first-pass test. If logic can be expressed as
an inline string, slice, AST node, or config struct, it belongs in a pure test.

### 2.2 Smallest Failing Test First

Each new behavior or extraction starts with the smallest failing test that proves
the behavior:

- threshold boundary -> one table row
- config default -> one assertion on the new field
- config validation -> one invalid input and one expected error
- scanner parse behavior -> one NDJSON fixture
- file walk behavior -> one temp tree with one relevant file

The first red test should never require a full repository fixture if a single
function call can expose the same failure.

### 2.3 Refactor Only Behind Green Tests

Package moves, file splits, router seam extraction, and helper consolidation are
refactors. Refactors only happen after the current test slice is green.

This matters especially for:

- splitting `security.go`
- splitting `quality.go`
- introducing provider-backed walk and scanner seams
- moving config load logic behind a host boundary

### 2.4 Behavior Preservation Before Improvement

Unless the design document explicitly changes a threshold or configuration
surface, tests should encode current behavior exactly. The rewrite is
reference-based. The old implementation is the behavioral oracle until a design
decision says otherwise.

---

## 3. Test Pyramid for This Rewrite

The rewrite should bias heavily toward fast unit tests and use integration tests
only where they buy real confidence.

### 3.1 Tier 1 - Pure Unit Tests

Use pure unit tests for:

- config defaulting and validation
- threshold calculations
- warning band classification
- secret pattern ranking and filtering
- allowlist and override behavior
- exported symbol token counting
- doc comment prefix checks
- package concern parsing from `doc.go`
- custom rule regex matching
- NDJSON parsing into `[]PolicyFact`

Characteristics:

- no disk
- no subprocess
- no router boot
- table-driven wherever thresholds or matrices exist
- `assert` as the default assertion style

### 3.2 Tier 2 - Minimal Integration Tests

Use integration tests for `Check*` orchestrators and any file-walk-dependent
behavior.

Characteristics:

- uses `t.TempDir()`
- creates the smallest possible real tree
- tests orchestration, not business logic exhaustively
- `require` for setup
- `assert` for returned violations and warnings

Examples:

- `CheckGoVersion` reading `go.mod`
- `CheckTestLocation` inspecting `_test.go` placement
- `CheckPackageRules` counting production files
- `CheckCustomRules` limiting matches by glob and extension

### 3.3 Tier 3 - Host Boundary Tests

Use host/provider boundary tests for:

- walk provider semantics
- scanner provider behavior
- config provider source resolution

These are not policy logic tests. They exist to ensure the policy engine gets
stable host behavior through ports.

### 3.4 Tier 4 - End-to-End Acceptance

End-to-end tests are sparse and deliberate:

- `go test ./internal/tests/... -v -count=1`
- `go run ./cmd/policycheck --policy-list`
- `go run ./cmd/policycheck`

The final acceptance criterion remains:

```powershell
go run ./cmd/policycheck --root . --config policy-gate.toml
```

Expected result:

```text
policycheck: ok
```

---

## 4. Repository Test Conventions

### 4.1 Required Tooling and Commands

The rewrite must stay aligned with repository policy:

```powershell
make build
make lint
go test ./internal/tests/... -v -count=1
python scripts/scanner_test.py -v
go run ./cmd/policycheck --policy-list
go run ./cmd/policycheck
```

During implementation, `go run ./cmd/policycheck` should be executed one to
three times, and always before completion.

### 4.2 Assertion Libraries

Use:

- `require` for setup and preconditions
- `assert` for normal expectations
- `suite` only where shared test state materially reduces duplication
- `mock` only if a scanner boundary truly needs it

Default to plain table-driven tests over `suite`.

### 4.3 Test Location

Go tests live under `internal/tests/`, mirroring the production layout.

Target layout for the rewrite:

```text
internal/tests/policycheck/
├── core/
│   ├── contracts/
│   ├── quality/
│   ├── security/
│   ├── hygiene/
│   ├── structure/
│   └── custom/
├── config/
├── host/
└── walk/
```

If the final package names differ slightly, keep the mirror principle intact.

### 4.4 Test Code Must Pass policycheck Too

Test code is not exempt from the standards enforced by the tool itself.

Keep test functions aligned with the repo rules:

- no swallowed errors
- low nesting
- helper extraction when setup becomes repetitive
- explicit, contextual failure messages where useful

Never do this in tests:

```go
_ = os.WriteFile(path, data, 0o644)
```

Always do this:

```go
require.NoError(t, os.WriteFile(path, data, 0o644))
```

---

## 5. TDD Workflow by Rewrite Layer

### 5.1 Config Layer

Start the rewrite with config because later phases depend on a stable config
surface.

Write these tests first:

- `defaults_test.go`
- `validation_test.go`
- `loader_test.go`

The first red tests should cover:

- new default values for added config structs
- invalid regex in `custom_rules`
- invalid threshold combinations
- empty or malformed required fields
- cross-field constraints such as file-size warn/max gap rules

Success condition:

- all new config structs exist
- defaults match the design document
- validation rejects invalid input with contextual errors
- regexes compile at config load time, not during policy execution

### 5.2 Pure Policy Logic Layer

Once config is stable, extract pure functions from each policy group and drive
them with unit tests.

Rule of thumb:

- if a function can accept `string`, `[]byte`, `[]PolicyFact`, AST nodes, or
  plain config structs, test it without touching disk

Write pure tests before extraction whenever possible. The failing test should
define the target API for the extracted function.

Examples:

- `validateGoVersion(content, allowedPrefixes)`
- `computeFileSizeThresholds(cfg, ctxFuncCount)`
- `evaluateFileSize(rel, lineCount, warnLOC, maxLOC)`
- `evaluateFunctionQualityFacts(facts, cfg)`
- `parseDocGoConcerns(content)`
- `hasAudienceModeSupport(content, requiredFlags)`
- `applySecretAllowlist(findings, cfg)`

### 5.3 Orchestration Layer

After pure logic is green, add minimal integration tests for the `Check*`
functions that:

- enumerate files
- read file content
- call scanners
- aggregate violations

These tests should prove only that the orchestrator wires the right inputs into
the already-tested pure logic.

The orchestrator test is complete when:

- the minimal fixture hits the intended code path
- one pass case exists
- one fail case exists
- no threshold matrix is duplicated from the unit tests

### 5.4 Host and Router Layer

The host layer is where regression risk rises because behavior moves behind
ports. Drive that work with seam tests.

Write tests for:

- walk provider traversal behavior
- scanner provider command/output handling
- config provider source loading

These tests are about host guarantees at the policycheck boundary, not router
internals or policy rules.

---

## 6. Phase-by-Phase TDD Plan

The rewrite phases from the design document become the execution order for tests.

### Phase 1 - Config Extension

Write failing tests for:

- new structs on `PolicyConfig`
- defaults for:
  - `GoVersion.AllowedPrefixes`
  - `Hygiene.*`
  - `PackageRules.*`
  - `AICompatibility.RequiredFlags`
  - `ScopeGuard.*`
- validation for:
  - invalid custom regex
  - invalid severity
  - invalid file-size threshold combinations
  - empty required config where prohibited

Then implement:

- struct additions
- default application
- validation rules
- config loader compilation of regexes

Phase completion gate:

- config tests green
- no behavior changes yet outside config load/validation

### Phase 2 - Walk and Scanner Host Capabilities

Write failing tests for:

- walking a directory tree through the provider
- filtering relevant file types through helper wrappers
- scanner output parsing from NDJSON
- timeout or execution failure surfacing as contextual errors
- no hidden global initialization for providers

Then implement:

- `WalkProvider`
- concrete walk adapter
- `ScannerProvider`
- concrete scanner adapter
- host seam wrappers consumed by policy packages

Phase completion gate:

- host tests green
- config tests still green
- existing policy behavior unchanged

### Phase 3 - Group Restructure and Pure Function Extraction

This is the longest phase. Work group by group, not file by file across the
entire codebase.

For each group:

1. write or expand pure-function tests
2. make them fail against the target extracted API
3. extract the pure function
4. make unit tests green
5. write minimal orchestrator integration tests
6. refactor package/file layout
7. run the full test suite for that slice

Recommended group order:

1. `contracts`
2. `quality`
3. `security`
4. `hygiene`
5. `structure`
6. `custom`

Reasoning:

- `contracts` is low-complexity and stabilizes simple seams early
- `quality` contains the highest-value threshold math and deserves early proof
- `security` is behavior-dense and easier after the two-layer pattern is
  established
- `custom` is independent and can land late without blocking core parity

Phase completion gate:

- split packages exist
- obsolete mixed-concern files are removed
- all unit and integration tests for migrated groups are green

### Phase 4 - Config Wire-Through

This phase proves that hardcoded values are actually gone.

For each formerly hardcoded behavior:

1. write a config-override test that should fail under the old hardcoded logic
2. implement the wire-through
3. keep the old default behavior green

Mandatory override-test targets:

- Go version allowed prefixes
- hygiene scan roots
- hygiene min name tokens
- package rules scan roots
- package rules thresholds
- AI compatibility required flags
- scope guard constant and forbidden call list
- function quality enabled languages
- secret benign hints
- secret placeholder strings

Phase completion gate:

- each hardcoded value has an explicit override test
- defaults still preserve historical behavior
- no hardcoded fallback silently bypasses config

### Phase 5 - Router Host Finalization

Treat this as a boundary-preservation phase, not a new-feature phase.

Write or finalize tests for:

- config, walk, and scanner resolution through ports
- policy packages consuming resolved providers through the intended host seam

Phase completion gate:

- provider resolution is router-backed end to end
- policy packages remain router-agnostic
- self-check still passes

---

## 7. Policy-by-Policy Test Matrix

The design document already defines the authoritative policy cases. The TDD
strategy here is to convert those cases directly into tests instead of
summarizing them loosely.

### 7.1 Contracts Group

#### Go Version

Pure tests:

- allowed prefix passes
- disallowed prefix fails
- missing `go` directive fails
- malformed `go.mod` returns contextual error

Override test:

- `allowed_prefixes = ["1.26"]` makes `go 1.26.0` pass

Integration test:

- temp repo with `go.mod` triggers one violation for unsupported version

#### CLI Formatter

Pure tests:

- required file with forbidden direct stdout pattern fails
- required file with audience-aware formatter passes
- `fmt.Sprintf` used for string construction passes

Integration test:

- only files listed in config are enforced

#### AI Compatibility

Pure tests:

- both required flags present passes
- missing one required flag fails
- wrapper resolution continues past thin wrapper file

Override test:

- `required_flags = ["--ai"]` accepts `--ai` without `--user`

#### Scope Guard

Pure tests:

- required constant present passes
- missing constant fails
- forbidden lifecycle call fails

Override tests:

- custom required constant changes the check
- empty forbidden call list disables those findings

### 7.2 Quality Group

#### File Size

Pure tests:

- every threshold row from the design table
- floor behavior
- warn/max gap enforcement
- line-count evaluation boundaries

Integration tests:

- ignored path prefix skipped
- file outside configured roots skipped
- oversized file returns correct severity

#### Function Quality

Pure tests:

- every CTX/LOC row from the design table
- nil guard repeat boundary
- recalibrated warning bands

Override test:

- `enabled_languages = ["go"]` skips Python and TypeScript facts

Integration tests:

- scanner facts route through orchestrator correctly

### 7.3 Security Group

#### Secret Logging

Pure tests:

- keyword detection
- representative regex detection per severity tier
- benign hint suppression
- placeholder suppression
- allowlist literal suppression
- allowlist pattern suppression
- override severity application
- severity ranking when multiple findings exist

Override tests:

- empty benign hints disables benign suppression
- custom placeholder strings alter suppression behavior

Integration tests:

- ignored path prefixes are not scanned
- files outside scan roots are not scanned

### 7.4 Hygiene Group

#### Symbol Names

Pure tests:

- token counting matrix from the design document
- acronym handling
- single-token exported names fail

Override tests:

- `min_name_tokens = 3` changes behavior

Integration tests:

- files outside scan roots are skipped

#### Doc Style

Pure tests:

- correct prefix comment passes
- incorrect prefix comment fails
- missing comment fails for exported symbol
- unexported symbol is ignored

Integration tests:

- generated and mock files are skipped

### 7.5 Structure Group

#### Test Location

Integration-first is acceptable here because behavior is path-based.

Tests:

- allowed test path passes
- disallowed test path fails
- non-test file ignored

#### Package Rules

Pure tests:

- concern parsing from `doc.go`
- concern count boundaries

Integration tests:

- production file count boundary
- `doc.go` required
- `_test.go` excluded from production counts

Override tests:

- `max_production_files`
- `min_concerns`
- `max_concerns`

#### Architecture Roots

Pure or integration depending on final implementation shape:

- allowed child passes
- disallowed child fails
- ignored child skipped
- `enforce = false` disables findings

### 7.6 Custom Group

Pure tests:

- valid regex + match -> violation
- valid regex + no match -> pass
- disabled rule -> skipped
- language mismatch -> skipped
- glob mismatch -> skipped

Config test:

- invalid regex rejected at load time

Integration test:

- one matching file in temp tree yields expected message and severity

---

## 8. Test Data Design

### 8.1 Prefer Inline Inputs First

If a test can be written with:

- a string literal
- a small `[]PolicyFact`
- a config struct
- a parsed AST from inline source

do that before creating files.

### 8.2 Temp Trees Must Stay Minimal

A good orchestration fixture usually has:

- one root
- one config
- one target file
- optionally one ignored sibling for contrast

If a fixture needs more than a few files, that usually means the test is
covering too many behaviors at once.

### 8.3 Avoid Opaque Golden Files Unless Necessary

Prefer explicit assertions over broad golden output snapshots. Snapshot-style
tests hide intent and make threshold refactors harder to review.

Golden inputs are acceptable for:

- NDJSON scanner output samples
- a representative `doc.go` concern block
- a deliberately malformed config sample

---

## 9. Router and Host Testing Guidance

The router is treated as complete infrastructure for this rewrite.

Test these guarantees explicitly:

- policycheck resolves config, walk, and scanner capabilities through the
  intended provider seam
- policy packages do not bypass the host seam with ad hoc direct wiring
- provider lookup failures surface contextual errors

Do not add tests whose only purpose is to re-prove router internals unless the
task is specifically about router implementation.

---

## 10. Suggested Command Cadence During Implementation

Use a predictable command cadence so regressions are caught while the change
surface is still small.

For each meaningful slice:

1. run the targeted test package
2. run `go test ./internal/tests/... -v -count=1` after the slice is green
3. run `go run ./cmd/policycheck` periodically during the phase
4. run `make lint` before closing a substantial phase

Recommended cadence by phase:

| Phase | Command focus |
|---|---|
| 1 | config package tests, then full Go tests |
| 2 | host/walk/scanner tests, then full Go tests |
| 3 | current group tests, then full Go tests, then `go run ./cmd/policycheck` |
| 4 | override tests, full Go tests, `go run ./cmd/policycheck` |
| 5 | full Go tests, `make lint`, `go run ./cmd/policycheck --policy-list`, `go run ./cmd/policycheck` |

---

## 11. Definition of Done

The rewrite is done only when all of the following are true:

- config additions are fully defaulted and validated
- every extracted pure function was introduced behind tests
- every `Check*` orchestrator has minimal integration coverage
- every formerly hardcoded behavior has an explicit config-override test
- walk, scanner, and config host capabilities are tested through their seams
- policycheck uses router-backed host capabilities correctly
- `go test ./internal/tests/... -v -count=1` passes
- `python scripts/scanner_test.py -v` passes
- `make build` passes
- `make lint` passes
- `go run ./cmd/policycheck --policy-list` passes
- `go run ./cmd/policycheck` passes
- the binary reports `policycheck: ok` against its own source root

Anything less is an intermediate checkpoint, not completion.

---

## 12. Practical Summary

The rewrite should be developed in this order:

1. config tests
2. host seam tests
3. pure function tests by group
4. minimal orchestrator tests by group
5. config-override tests to remove hardcoding
6. router finalization tests
7. full self-check

The key discipline is to keep every test slice small, direct, and local to the
behavior being changed. If the rewrite follows that discipline, the package
restructure becomes mechanically safe instead of risky.
