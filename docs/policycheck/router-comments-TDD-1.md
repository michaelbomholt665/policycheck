# Router Comments TDD 1

## Objective

Evolve the configurable `documentation` policy so it enforces file-level headers and function-level documentation across Go, Python, and TypeScript with a stable two-level model:

- **Loose**: existence and basic placement checks
- **Strict**: style-specific compliance for analytics-ready code

This document is no longer a pure greenfield TDD spec. Parts of the rule surface, defaults, validation, registry wiring, and tests already exist. The remaining work is to align the implementation with the refined style matrix, strict/loose semantics, and higher-signal diagnostics captured below.

## Current State

Already present in the repo:

- `[documentation]` config section in `policy-gate.toml`
- `PolicyDocumentationConfig` in `internal/policycheck/config/config_manager.go`
- Rule registration in `internal/policycheck/core/policy_registry.go`
- Implementation scaffold in `internal/policycheck/core/hygiene/documentation.go`
- Documentation tests under `internal/tests/policycheck/core/hygiene/documentation_test.go`
- Config defaults and validation tests

Current implementation constraints discovered during review:

- `level` already supports `loose` and `strict`
- Current allowed style values are still too narrow and inconsistent with the intended design
- Violation messages are too generic; they do not consistently include configured strictness/style or exact expected shape
- Python shebang handling is currently global; it should become path-aware
- Relative path drift detection is mandatory and must stay mandatory

## Scope

- Preserve and refine the existing `[documentation]` policy section.
- Enforce file headers:
  - repo-relative path
  - 2-5 line module/file description
  - mandatory path drift detection
- Enforce function documentation:
  - loose mode: attached documentation must exist
  - strict mode: selected style must be satisfied
- Support a broader style matrix per language.
- Improve violation messages so they report level, configured style, and exact expected form.
- Keep multi-language AST parsing through the current Go-native and scanner-backed approach.
- Expand tests under `internal/tests/`.

## Non-Negotiables

- [x] Read `AGENTS.md` before implementation.
- [x] **FORBIDDEN**: Do not modify any files in `internal/router/`.
- [x] **IMPORTS**: Adapters may ONLY import `policycheck/internal/ports` and `policycheck/internal/router`.
- [x] **IMPORTS**: Business logic and features MUST NOT import adapters; they must use the router.
- [x] Enforce relative path accuracy (path in comment must match actual file path).
- [x] Relative path drift detection is mandatory in both `loose` and `strict`.
- [x] Python shebang enforcement must be path-aware; do not require a shebang for every Python file globally.
- [x] Strict mode must enforce the selected per-language style, not a single hardcoded style per language.
- [x] Use existing AST providers; do not re-implement file walking.
- [x] Keep the rule configurable via `policy-gate.toml`.

## Router And Adapter Boundary Rules

These rules are mandatory for this work. The documentation policy must not bypass or weaken the router architecture.

- Treat the router as the only dependency boundary between business logic and concrete providers.
- Business logic may import:
  - `policycheck/internal/ports`
  - `policycheck/internal/router`
  - `policycheck/internal/router/capabilities`
- Business logic must not import:
  - `policycheck/internal/adapters/...`
  - `policycheck/internal/router/ext/...`
- Adapters must not import other adapters to reach a capability. If an adapter needs another capability, expose a port contract and resolve it through the router boundary.
- Router core is frozen infrastructure. Do not redesign router internals as part of documentation-rule work.
- Router wiring remains the only place where concrete providers are connected:
  - required application wiring: `internal/router/ext/extensions.go`
  - optional capability wiring: `internal/router/ext/optional_extensions.go`
- Do not introduce parallel bootstrapping, side registries, or direct provider wiring for policycheck.
- Use existing host capabilities for config, walking, and scanner access instead of bypassing router-backed seams.
- Tests for this work should verify policycheck behavior through resolved providers and scanner seams, not re-prove router internals.

### Import Rule Summary

- consumer code -> `internal/ports` + `internal/router`
- router-native typed capability access -> `internal/router/capabilities`
- host boot code -> `internal/router/ext`
- concrete provider implementations -> `internal/adapters/*`

### Anti-Tangle Rules

- Do not let documentation validation import adapter packages just to access scanner/config/walk functionality.
- Do not move scanner or host responsibilities into the hygiene rule package.
- Do not make adapters aware of each other to satisfy documentation checks.
- If a new host capability is needed, add it through the existing router wiring path rather than adding a direct dependency edge.

## Intended Config Shape

```toml
[documentation]
enabled = true
level = "strict" # "loose" or "strict"
scan_roots = ["internal", "cmd", "scripts"]

# Style Selection
# Go: google | standard | presence_only
# Python: google | numpy | restructuredtext | standard | presence_only
# TypeScript: tsdoc | jsdoc | standard | presence_only
go_style = "google"
python_style = "numpy"
typescript_style = "tsdoc"

# Specific Enforcements
enforce_headers = true
enforce_functions = true
require_shebang_python = true
python_shebang_roots = ["scripts"]
```

## Config Semantics

### Level Semantics

`loose`

- Header path must exist near the top and match the actual repo-relative path.
- Header description must provide 2-5 lines of content.
- Functions must have an attached documentation block/comment/docstring.
- No exact tag/section enforcement.
- No whitespace/aesthetic enforcement beyond basic attachment and placement.

`strict`

- Header position is exact by language.
- Header path must exactly match the repo-relative path.
- Header description must occupy the required lines.
- Function documentation must satisfy the configured style for that language.
- Required tags/sections must reflect the function signature.
- Style-specific structure and formatting rules apply.

### Header Layout

Loose mode:

- Go/TypeScript: path must appear in the first few lines and match the actual relative path.
- Python: path must appear near the top and match the actual relative path.
- Description must be 2-5 comment/doc lines after the path header.

Strict mode:

- Go/TypeScript:
  - Line 1: relative path
  - Lines 2-6: 2-5 line description
- Python:
  - Line 1: shebang only when the file is under configured executable roots
  - Next required header line: relative path
  - Following 2-5 lines: module description

### Style Matrix

Go:

- `google`: summary starts with the function name; if additional detail exists, use the standard blank comment separator before the following paragraph
- `standard`: regular Go doc comment expectations without the stronger Google-specific shape checks
- `presence_only`: any attached documentation comment passes

Python:

- `google`: summary plus structured sections such as `Args:`, `Returns:`, `Raises:`
- `numpy`: NumPy/SciPy-style docstring with underlined sections such as `Parameters` and `Returns`
- `restructuredtext`: reST/Sphinx-style fields such as `:param name:` and `:returns:`
- `standard`: non-trivial attached docstring with no tagged style requirement
- `presence_only`: any attached docstring passes

TypeScript:

- `tsdoc`: `/** ... */` block with `@param`, `@returns`, and other tags as required
- `jsdoc`: JSDoc-compatible block semantics
- `standard`: non-trivial attached documentation block with no tag requirement
- `presence_only`: any attached documentation comment passes

### Quality Floor

Loose mode should not attempt to score "meaning", but strict mode may apply light anti-filler checks:

- minimum summary length
- reject obviously weak openers such as `This function`, `This method`, or `Function to`
- for Go, reject comments that only restate the symbol name without adding information

These checks should remain conservative. The rule should optimize for high-signal enforcement, not speculative semantic grading.

## Task Checklist

- [x] Add `PolicyDocumentationConfig` to `internal/policycheck/config/config_manager.go`.
- [x] Add defaults and validation for the new config section.
- [x] Implement `internal/policycheck/core/hygiene/documentation.go`.
- [x] Register the rule in the policy registry.
- [x] Add focused tests under `internal/tests/`.
- [x] Expand style validation to match the broader per-language style matrix.
- [x] Separate `standard` and `presence_only` semantics from strict style enforcement.
- [x] Make Python shebang enforcement root-aware via configured executable roots.
- [x] Tighten strict header positioning while keeping loose header checks placement-tolerant.
- [x] Improve diagnostics to include:
  - configured `level`
  - configured language style
  - exact expected shape
  - actual mismatch when available
- [x] Extend scanner-backed checks as needed for `google`, `numpy`, `restructuredtext`, `tsdoc`, and `jsdoc`.
- [x] Add regression tests for generic-vs-style-specific diagnostics.
- [x] Keep all documentation-rule changes within existing router and host seams; no direct adapter imports from business logic.
- [x] Add or update tests that prove policycheck uses router-resolved scanner access rather than direct adapter coupling.

## File Plan

| File |
| --- | --- | --- |
| `internal/policycheck/config/config_manager.go` | update | Expand allowed styles, add path-aware Python shebang config, preserve defaults |
| `internal/policycheck/core/hygiene/documentation.go` | update | Refine loose/strict semantics, drift detection, diagnostics, style validators |
| `internal/policycheck/core/policy_registry.go` | existing | Rule already registered; update only if rule metadata needs revision |
| `scripts/scanner.py` | update | Ensure Python docstring extraction supports Google, NumPy, and reST validation |
| `scripts/scanner.ts` | update | Ensure TS doc extraction supports TSDoc and JSDoc validation |
| `internal/tests/policycheck/core/hygiene/documentation_test.go` | update | Add style matrix, strictness, header drift, and diagnostic assertions |
| `internal/tests/policycheck/config/validation_test.go` | update | Validate new style options and Python shebang roots config |
| `policy-gate.toml` | update | Keep documentation config aligned with supported styles |

## TDD Cycles

### T1 Config & Surface [ ]
RED:
- [x] Write a failing test for invalid expanded style values per language.
- [x] Write a failing test for invalid Python shebang root configuration if roots are malformed.

GREEN:
- [x] Expand the allowed style set:
  - Go: `google`, `standard`, `presence_only`
  - Python: `google`, `numpy`, `restructuredtext`, `standard`, `presence_only`
  - TypeScript: `tsdoc`, `jsdoc`, `standard`, `presence_only`
- [x] Add `python_shebang_roots` with a sensible default such as `["scripts"]`.
- [x] Keep `level = "loose"` as the default.

### T2 File Header Enforcement [ ]
RED:
- [x] Write a failing test for path drift where the header path does not match the actual relative path.
- [x] Write a failing test for a missing path header at the required strict position.
- [x] Write a failing test for a 1-line or 6-line module description.
- [x] Write a failing test for a Python executable file missing a shebang in Strict mode.

GREEN:
- [x] Implement mandatory repo-relative path equality checks in both modes.
- [x] Implement strict header line-position rules by language.
- [x] Count continuous comment lines following the path to validate description length.
- [x] Limit shebang enforcement to configured executable roots.

### T3 Function Documentation (Loose) [ ]
RED:
- [x] Write a failing test for an undocumented function in a Go file.
- [x] Write a failing test for an undocumented function in a Python file.
- [x] Write a failing test for an undocumented function in a TypeScript file.

GREEN:
- [x] Check `ast.FuncDecl.Doc` in Go.
- [x] Check extracted `docstring` field from Python/TS scanners.
- [x] Flag missing documentation only.

### T4 Function Documentation (Strict - Go/Google) [x]
RED:
- [x] Write a failing test for a Go comment that doesn't start with the function name.
- [x] Write a failing test for a multi-paragraph Go comment that omits the blank separator comment line.

GREEN:
- [x] Validate that `strings.HasPrefix(comment, funcName)`.
- [x] Enforce the blank separator line when a detailed paragraph follows.

### T5 Function Documentation (Strict - Python Styles) [x]
RED:
- [x] Write a failing test for a Python Google docstring missing `Args:` or `Returns:`.
- [x] Write a failing test for a Python docstring missing the `Parameters` or `Returns` section headers.
- [x] Write a failing test for a Python reST docstring missing `:param` or `:returns:`.

GREEN:
- [x] Validate Google-style sections when `python_style = "google"`.
- [x] Use regex to find `Parameters\n----------` and `Returns\n-------` in Python docstrings.
- [x] Validate reST/Sphinx field syntax when `python_style = "restructuredtext"`.

### T6 Function Documentation (Strict - TS Styles) [x]
RED:
- [x] Write a failing test for a TS function missing `@param` for a declared argument.
- [x] Write a failing test for a TS style mismatch where `tsdoc` is configured but `//` is used instead of a block comment.

GREEN:
- [x] Compare `ast.FuncDecl` parameter names against `@param` tags in the doc block.
- [x] Require `/** ... */` block form for `tsdoc`.
- [x] Validate `jsdoc` using its accepted block/tag conventions.

### T7 Diagnostics [ ]
RED:
- [x] Write a failing test for a violation message that omits configured `level`.
- [x] Write a failing test for a violation message that omits configured style.
- [x] Write a failing test for a path/shebang/style violation message that does not explain the exact expected form.

GREEN:
- [x] Emit diagnostics in the form:
  - `<location>: <subject> <failed-condition> (level=<level>, <lang>_style=<style>); expected <expectation> [hygiene.documentation]`
- [x] Include actual path mismatches when available.
- [x] Include shebang expectations and style-specific section/tag expectations.

## Suggested Violation Messages

- `internal/tests/policycheck/host/ports_test.go:SetPath:19: function "SetPath" is missing documentation (level=loose, go_style=google); expected a doc comment immediately above the function [hygiene.documentation]`
- `internal/tests/policycheck/host/ports_test.go:SetPath:19: function "SetPath" violates documentation style (level=strict, go_style=google); expected the summary line to start with "SetPath" [hygiene.documentation]`
- `scripts/scanner.py:1: missing required shebang (level=strict, python_style=numpy); expected "#!/usr/bin/env python3" on line 1 for files under configured executable roots [hygiene.documentation]`
- `internal/tests/policycheck/host/ports_test.go:1: file header path is incorrect (level=strict); expected "internal/tests/policycheck/host/ports_test.go" on line 1, found "internal/tests/policycheck/ports_test.go" [hygiene.documentation]`
- `scripts/scanner.py:42: function "calculate_risk" violates documentation style (level=strict, python_style=numpy); missing required "Parameters" section [hygiene.documentation]`
- `internal/ui/render.ts:init:18: function "init" violates documentation style (level=strict, typescript_style=tsdoc); missing @param tag for argument "config" [hygiene.documentation]`

## Verification

- [x] `go test ./internal/tests/policycheck/core/hygiene/... -count=1`
- [x] `go test ./internal/tests/policycheck/config/... -count=1`
- [x] `go run ./cmd/policycheck`
- [x] `go run ./cmd/policycheck --policy-list`
