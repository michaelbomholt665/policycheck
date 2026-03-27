# Design Document — `policycheck` CLI Wrapper

---

**Status:** Draft  
**Component:** `policycheck` — CLI wrapper / command dispatcher  
**Language:** Go  
**Target OS:** Linux, macOS, Windows (WSL2 primary during development)  
**Config Format:** TOML  
**Author:** Michael  
**Date:** 2026-03-25

## Implementation Reconciliation Note (2026-03-27)

The current implementation intentionally ships a reduced interaction surface for the wrapper MVP:

- Moderate-risk package findings are block-or-override only; interactive TTY prompts are deferred.
- Macro execution uses the shared `internal/cliwrapper` core, but `run --dry-run` and prompted template-variable collection are deferred.
- `fmt headers --dry-run` is implemented and CI-safe; it exits non-zero when files would change.
- Package-block output is intentionally plain wrapper error text with an explicit `--allow-risk=<level>` hint, rather than the richer styled advisory rendering sketched below.

These reductions keep the shared binary and router-linking surface stable while the remaining UX work is completed in follow-up phases.

---

## 1. Overview

`policycheck` is a PATH-resident CLI binary written in Go. It acts as a **command interceptor and workflow dispatcher** that sits in front of arbitrary shell commands. It does not replace those commands — it wraps them with pre- and post-execution policy gates, enforcing security and code-quality rules before delegating to the underlying tool.

This document covers the **wrapper surface only**: the command dispatch model, security gate (OSV), tooling gate, workflow macros, and dual-layer config system. The style/complexity analysis engine is out of scope here.

---

## 2. Problem Statement

Running `uv add somepackage` or `go get somemodule` in a repo currently bypasses any dependency vetting. Similarly, running formatters and test suites in the wrong order, or forgetting to push after a commit, are sources of CI failures and developer friction. There is no single enforcer that lives in the developer's shell and is aware of both security policy and project-level workflow conventions.

`policycheck` solves this by becoming the single entry point for any command that should be gated.

---

## 3. Core Design Principles

- **Explicit over implicit.** Gates either pass or block — no silent degradation.
- **Fail-loud.** If a gate check cannot complete (OSV unreachable, config malformed), the command does not run and the reason is printed.
- **Opt-in escalation only.** A blocked command can only be force-run via an explicit flag (`--allow-risk=<level>`), never silently.
- **CWD-aware.** All commands execute relative to the current working directory. No assumptions about project root are made without resolution logic (walk up for `policycheck.toml`).
- **Config layering is additive, not overriding.** Global config provides defaults and macros; repo config refines or extends them. Repo config never silently inherits a global setting it didn't opt into.

---

## 4. Command Surface

```
policycheck <subcommand|passthrough> [args...]

policycheck uv add fastapi          # package gate → install
policycheck pip install httpx       # package gate → install
policycheck go get github.com/x/y   # package gate → install
policycheck bun add zod             # package gate → install

policycheck gofumpt ./... && go test ./...   # tooling gate (sequential)
policycheck run fmt                 # named workflow macro
policycheck run release             # named workflow macro

policycheck check                   # run policy checks on current repo (existing surface)
policycheck config                  # inspect/validate config
policycheck config --global         # inspect global config
policycheck config init             # scaffold repo config
policycheck config init --global    # scaffold global config

policycheck fmt headers             # inject path-comment headers into all supported files
policycheck fmt headers --dry-run   # preview changes without writing
policycheck fmt headers --dry-run --list   # preview exactly which files would change
policycheck fmt headers --only go   # restrict to one or more languages
policycheck fmt headers --only python typescript
```

---

## 5. Feature Specifications

---

### 5.1 Package Security Gate (OSV Integration)

**Invocation pattern:**
```
policycheck <pkg-manager> add|install|get|add [package[@version]...]
```

**Supported package managers and ecosystems:**

| Manager  | Ecosystem tag (OSV) | Language   |
| -------- | ------------------- | ---------- |
| `pip`    | PyPI                | Python     |
| `uv`     | PyPI                | Python     |
| `poetry` | PyPI                | Python     |
| `npm`    | npm                 | TypeScript |
| `pnpm`   | npm                 | TypeScript |
| `bun`    | npm                 | TypeScript |
| `go get` | Go                  | Go         |

**OSV Scanner version:** v2.3.5+ (v2 CLI syntax throughout — v1 flags are incompatible)

**OSV binary resolution order:**
1. `osv-scanner` in PATH (native Linux binary)
2. `osv-scanner.exe` in PATH (Windows binary via WSL2 interop fallback)
3. OSV REST API (`https://api.osv.dev/v1/query`) — CI fallback when no binary present

**Gate flow:**

```
parse packages from args
  │
  ├─► Phase 1: pre-install scan (purl — known package + version)
  │       osv-scanner scan --format json --package "pkg:<eco>/<name>@<ver>"
  │
  ├─► no vulns → run install command
  │
  ├─► vulns found
  │     ├─► CRITICAL / HIGH ──► block, print reason + override hint, exit 1
  │     │                        └─► --allow-risk=high to override
  │     ├─► MODERATE ───────► block or override (interactive prompt deferred)
  │     └─► LOW / INFO ─────► warn only, proceed
  │
  └─► Phase 2: post-install lockfile scan (catches transitive deps)
          osv-scanner --lockfile=<resolved-lockfile> --format json
          same severity gate as Phase 1
          on block: advise manual removal of installed package
```

**Phase 2 lockfile resolution by ecosystem:**

| Ecosystem | Lockfile                                           |
| --------- | -------------------------------------------------- |
| PyPI      | `uv.lock`, `poetry.lock`, `requirements.txt`       |
| npm       | `package-lock.json`, `bun.lockb`, `pnpm-lock.yaml` |
| Go        | `go.sum`                                           |

**Full OSV v2 command reference** (decide which apply at implementation time):

```bash
# Pre-install: single package via purl
osv-scanner scan --format json --package "pkg:pypi/fastapi@0.111.0"
osv-scanner scan --format json --package "pkg:npm/zod@3.22.0"
osv-scanner scan --format json --package "pkg:golang/github.com/some/pkg@v1.2.0"

# Post-install: lockfile scan
osv-scanner --lockfile=uv.lock --format json
osv-scanner --lockfile=go.sum --format json
osv-scanner --lockfile=package-lock.json --format json
osv-scanner --lockfile=bun.lockb --format json
osv-scanner --lockfile=pnpm-lock.yaml --format json

# SBOM scan (future — if repo generates SBOM)
osv-scanner --sbom=cyclonedx-or-spdx-sbom.json --format json

# Recursive directory scan (repo-wide audit, future `policycheck audit` surface)
osv-scanner -r path/to/project --format json

# Guided remediation — non-interactive (future `policycheck fix` surface)
osv-scanner fix --non-interactive --strategy=in-place -L path/to/package-lock.json
osv-scanner fix --non-interactive --strategy=relock -M path/to/package.json -L path/to/package-lock.json

# Guided remediation — interactive (future TUI surface)
osv-scanner fix -M path/to/package.json -L path/to/package-lock.json

# Container image scan (out of scope for wrapper MVP)
osv-scanner scan image --serve alpine:3.12
```

**Current output shape (MVP):**
```
dispatcher: package gate: pre-install scan: critical vulnerability in lodash; use --allow-risk=critical to override
```

**Future styled output example:**
```
[policycheck] ⚑ OSV scan: fastapi@0.111.0
[policycheck] ✗ CRITICAL  CVE-2024-XXXX  fastapi  path: starlette → httpx
[policycheck]   Advisory: https://osv.dev/vulnerability/CVE-2024-XXXX
[policycheck] Command blocked. Use --allow-risk=critical to override (not recommended).
```

**Design note:** Package version is resolved from the argument if provided (e.g. `fastapi==0.111.0`). If no version is specified, `policycheck` queries OSV for the latest known version before install — best-effort only, pinned-version accuracy is higher. The two-phase approach (purl pre-check + lockfile post-check) is the only way to catch vulnerable transitive dependencies that a direct package scan won't see.

---

### 5.2 Tooling Gate (Sequential Command Chain)

**Problem:** Running `go test ./...` without first running `gofumpt` (or `goimports`, `staticcheck`, etc.) means test output can reflect unformatted code, muddying CI signal.

**Invocation pattern:**
```
policycheck <gate-command> [gate-args] -then <main-command> [main-args]
```

Or via config-defined tooling pairs (see §6).

**Gate flow:**

```
run gate-command
  ├─► exit 0 ──► run main-command
  └─► exit non-0 ──► block main-command, print gate failure output, exit 1
```

**Examples:**
```bash
policycheck gofumpt -l ./... -then go test ./...
policycheck eslint src/ -then bun test
policycheck mypy src/ -then pytest
```

**Config-driven shorthand** (see §6):
```toml
[tooling.gates]
go-test = { gate = "gofumpt -l ./...", run = "go test ./..." }
py-test = { gate = "mypy src/", run = "pytest" }
```

Then: `policycheck run go-test`

**Design note:** The `-then` separator was chosen deliberately over `&&` because shell `&&` is evaluated by the shell before `policycheck` sees it. `-then` is unambiguous and fully owned by the dispatch layer.

---

### 5.3 Workflow Macros (`run` subcommand)

**Purpose:** Named, multi-step command sequences configured globally or per-repo. Reduces repetitive multi-command invocations to a single `policycheck run <name>`.

**Planned invocation:**
```
policycheck run <macro-name> [--dry-run] [--verbose]
```

**Current invocation (MVP):**
```
policycheck run <macro-name>
policycheck <macro-name>
```

**Macro definition (TOML):**
```toml
[macros.commit-push]
description = "Stage all, commit with message, push to origin/main"
steps = [
  "git add -A",
  'git commit -m "{{.message}}"',    # template variable — prompted at runtime
  "git push origin main"
]
on_failure = "stop"   # stop | continue

[macros.release-py]
description = "Format, type-check, test, then build and publish"
steps = [
  "ruff format .",
  "mypy src/",
  "pytest",
  "uv build",
  "uv publish"
]
on_failure = "stop"
```

**Template variables:** Prompted runtime collection is deferred in the MVP. Macros currently require all template variables to be supplied by the caller through the loaded config/runtime variable map; missing variables fail the run with an explicit error.

**`--dry-run`:** Deferred for macro runs in the MVP. `fmt headers --dry-run` is implemented; macro dry-run is a follow-up item.

**`on_failure` semantics:**
- `stop` — halt at failing step, terminate all child processes spawned by that step, report error, exit 1 *(default)*
- `continue` — log failure, terminate child processes of the failed step, continue remaining steps

**Process cleanup invariant:** On any failed or blocked run — macro step, gate block, or `-then` chain failure — `policycheck` must terminate all child processes it spawned before exiting. No ghost processes. This applies universally:
- Macro: step N fails → kill step N subprocess → halt (stop) or continue to step N+1
- Tooling gate: gate command fails → main command is never started; if main command is mid-run and killed externally → its process group is cleaned up
- Package gate: OSV blocks → package manager process is never spawned

Implementation note: use process groups (`syscall.SysProcAttr{Setpgid: true}` on Linux/macOS) so that children-of-children are also reaped. On Windows, use Job Objects or `cmd.Process.Kill()` directly as pgid semantics do not apply. Wire cleanup into a deferred handler so it fires on both normal exit and signal (SIGINT, SIGTERM).

**Design note:** Macros are intentionally not Makefile targets or shell aliases. They live in `policycheck` config and are therefore version-controllable (repo macros) or machine-local (global macros). This matters for team enforcement.

---

### 5.4 Global + Repo Config Layering

**Two config files, two scopes:**

| File                                | Location                                                                         | Scope                                         |
| ----------------------------------- | -------------------------------------------------------------------------------- | --------------------------------------------- |
| `~/.config/policycheck/config.toml` | `$XDG_CONFIG_HOME` (Linux/macOS) / `%APPDATA%\policycheck\config.toml` (Windows) | Machine-global defaults, global macros        |
| `<repo-root>/policy-gate.toml`      | Nearest ancestor dir containing this file                                        | Repo-specific policy, overrides, local macros |

**Resolution:** `policycheck` walks upward from CWD to find `policy-gate.toml`. If not found, only global config applies. Both files can coexist — repo config extends global config.

**Merge semantics:**
- Scalar values: repo wins over global
- `[macros.*]`: merged by key; repo macro with same name as global macro overrides it
- `[tooling.gates]`: merged by key
- `[security]` thresholds: repo can only be **stricter** than global (not more permissive). A repo attempting to lower a global block threshold is a config validation error.

**Global config skeleton (`~/.config/policycheck/config.toml`):**
```toml
[security]
block_on = ["CRITICAL", "HIGH"]   # severity levels that hard-block
warn_on  = ["MODERATE"]
allow_on = ["LOW", "INFO"]
osv_mode = "cli"                  # "cli" | "api"

[tooling]
# globally defined gate pairs (can be overridden per repo)

[macros]
# global macros available in any repo

[ui]
color = true
verbose = false
```

**Repo config skeleton (`policy-gate.toml`):**
```toml
[meta]
project = "syntx"
language = ["go", "python"]

[security]
# override: also block MODERATE in this repo
block_on = ["CRITICAL", "HIGH", "MODERATE"]

[tooling.gates]
go-test = { gate = "gofumpt -l ./...", run = "go test ./..." }

[macros.db-migrate]
description = "Run DB migrations then restart dev server"
steps = [
  "goose up",
  "air"
]
on_failure = "stop"
```

---

### 5.5 `fmt` — Repository File Formatting

**Purpose:** Repo-wide file manipulation that is not a gate. `fmt` is the home for any command that enforces structural conventions across source files. `headers` is the first subcommand; license banners, import ordering, or other file-level enforcement can be added here later without touching the gate machinery.

---

#### 5.5.1 `fmt headers` — Path Comment Injection

**Invocation:**
```
policycheck fmt headers [--dry-run] [--list] [--only <lang>...]
```

**Purpose:** Walk the repository root and ensure every Go, Python, and TypeScript file carries the correct path-comment header. Idempotent — safe to re-run and safe to wire into a pre-commit hook or CI check.

**Header format by language:**

| Language         | Format                                         | Position  |
| ---------------- | ---------------------------------------------- | --------- |
| Go               | `// path/to/file.go`                           | Line 1    |
| TypeScript / TSX | `// path/to/file.ts`                           | Line 1    |
| Python           | `#!/usr/bin/env python3` + `# path/to/file.py` | Lines 1–2 |

All paths are relative to the resolved repo root (same root resolution as `policy-gate.toml` walk).

**Flags:**

| Flag               | Description                                              |
| ------------------ | -------------------------------------------------------- |
| `--dry-run`        | Print what would change without writing any files        |
| `--list`           | Print the repo-relative files that would be modified     |
| `--only <lang>...` | Restrict to one or more of: `go`, `python`, `typescript` |

**Skip list (never modified):**
`.git`, `.venv`, `venv`, `node_modules`, `__pycache__`, `.mypy_cache`, `.ruff_cache`, `dist`, `build`, `vendor`

**Detection logic:** Scans the first 3 lines of each file for the path comment. If found and correct, file is skipped. If missing or stale (path has changed), header is injected or corrected at line 1 (Go/TS) or lines 1–2 (Python, preserving or adding shebang).

**Output:**
```
[policycheck fmt headers] scanning: /home/michael/syntx
  ADDED    internal/command/chain.go
  ADDED    internal/command/argparse.go
  STALE    internal/ports/macro_port.go   (was: internal/macro/macro_port.go)
  SKIPPED  internal/config/loader.go      (header present)

Checked : 47
Modified: 3
Skipped : 44
```

**CI usage pattern:**
```bash
policycheck fmt headers --dry-run
# exits 1 if any file would be modified — fail the pipeline

policycheck fmt headers --dry-run --list
# prints the exact repo-relative files that would be modified, then exits 1 if any are pending
```

**Architecture placement:**

- Port: `internal/ports/fmt_port.go` — `HeaderFormatter` interface
- Adapter: `internal/adapters/fmt/` — `headers.go` + `doc.go`
- Logic: `internal/fmt/` — `walker.go`, `header.go`, `doc.go`

---

---

## 6. Architecture

### 6.1 Hexagonal Layout

`policycheck` follows the same hexagonal (ports & adapters) architecture used across the broader Syntx ecosystem. Adapters wire into the router — never into each other. `main.go` does not import adapters directly; wiring is declared via `wrlk guide` / `wrlk guide current`.

```
policycheck/
├── main.go                                  # loads router, reads wrlk wiring — NO adapter imports
├── internal/
│   │
│   ├── ports/
│   │   ├── doc.go
│   │   ├── command_port.go                  # CommandDispatcher interface
│   │   ├── security_port.go                 # SecurityGate interface (OSV)
│   │   ├── macro_port.go                    # MacroRunner interface
│   │   ├── config_port.go                   # ConfigLoader interface
│   │   └── fmt_port.go                      # HeaderFormatter interface
│   │
│   ├── adapters/
│   │   ├── doc.go
│   │   ├── command/                         # package gate + tooling gate + passthrough
│   │   │   ├── doc.go
│   │   │   ├── detector.go                  # classifies args → gate type
│   │   │   └── executor.go                  # spawns subprocesses, manages process groups
│   │   ├── security/                        # OSV integration
│   │   │   ├── doc.go
│   │   │   ├── osv_cli.go                   # osv-scanner subprocess (primary)
│   │   │   └── osv_api.go                   # OSV REST API (fallback)
│   │   ├── macro/                           # workflow macro runner
│   │   │   ├── doc.go
│   │   │   └── runner.go                    # step executor, on_failure, template vars
│   │   └── fmt/                             # fmt subcommand adapter
│   │       ├── doc.go
│   │       └── headers.go                   # walks repo, injects/corrects path comments
│   │
│   ├── command/                             # core business logic — no adapter imports
│   │   ├── doc.go
│   │   ├── chain.go                         # -then chain logic + process cleanup invariant
│   │   ├── argparse.go                      # parse pkg names/versions/ecosystems from args
│   │   ├── severity.go                      # severity classification and gate decisions
│   │   └── template.go                      # {{.var}} interpolation + runtime prompt
│   │
│   ├── fmt/                                 # fmt business logic — no adapter imports
│   │   ├── doc.go
│   │   ├── walker.go                        # recursive repo walk, skip list, extension filter
│   │   └── header.go                        # header detection, injection, stale correction
│   │
│   └── config/
│       ├── doc.go
│       ├── loader.go                        # load + merge global and repo configs
│       ├── resolver.go                      # CWD → repo root walk (finds policy-gate.toml)
│       └── schema.go                        # TOML schema structs
│
└── policy-gate.toml                         # repo config for policycheck's own development
```

**Adapter import rules (hard constraints):**
- Adapters import `internal/router` and `internal/ports` only
- Adapters never import another adapter
- `main.go` never imports adapters — wiring declared via `wrlk`
- Every new package gets a `doc.go`

**UI:** Lipgloss + Huh for all terminal output and interactive prompts. No custom printer layer.

**Framework:** `cobra` for CLI surface. OSV subprocess via `os/exec`; API fallback via `net/http` stdlib only.

### 6.2 Command Classification (Detector)

Lives in `internal/adapters/command/detector.go`. First thing called after arg parsing:

```
args[0] match against known pkg-manager names?
  yes → PackageGate (security adapter)
  no  → args contain "-then"?
          yes → ToolingGate (command adapter chain)
          no  → macro name match in config?
                  yes → MacroRunner (macro adapter)
                  no  → bare passthrough (executor, no gate)
```

Bare passthrough emits a subtle indicator so it is never ambiguous whether the wrapper was active:
```
[policycheck] passthrough: no gate matched for "git status"
```

---

## 7. Open Questions / Design Decisions Pending

| #   | Question                                                  | Status      | Decision / Notes                                                                                                                                     |
| --- | --------------------------------------------------------- | ----------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | OSV CLI vs API as primary                                 | **Decided** | CLI primary (`osv-scanner` / `osv-scanner.exe`); API fallback for CI environments without binary                                                     |
| 2   | `-then` separator                                         | **Decided** | `-then` confirmed — no conflict with bash, zsh, pwsh, fish. Also supported via `[tooling.gates]` config for repeatable use                           |
| 3   | Non-interactive mode for MODERATE blocks                  | **Decided** | Auto-block in non-TTY (CI); prompt y/n in TTY. Detect via `os.Stdin` isatty                                                                          |
| 4   | `rollback` macro semantics                                | **Decided** | Deferred to v2. MVP supports `stop` and `continue` only. Rollback requires compensating actions that are domain-specific and error-prone to automate |
| 5   | Global config location                                    | **Decided** | `$XDG_CONFIG_HOME/policycheck/config.toml` on Linux/macOS; `%APPDATA%\policycheck\config.toml` on Windows                                            |
| 6   | Repo config filename                                      | **Decided** | `policy-gate.toml` — visible (no dot-prefix), clearly identifying                                                                                    |
| 7   | OSV two-phase gate (purl pre-check + lockfile post-check) | **Pending** | Two-phase approach catches transitive deps; decide whether post-install lockfile scan is always-on or opt-in per repo config                         |
| 8   | OSV batch queries for multi-package installs              | **Pending** | Verify whether `osv-scanner scan --package` accepts multiple `--package` flags in one invocation to avoid N subprocess calls for `uv add a b c`      |
| 9   | `osv-scanner fix` surface                                 | **Pending** | Guided remediation commands exist in v2; decide if `policycheck fix` exposes this or stays out of scope for wrapper MVP                              |

---

## 8. Out of Scope (This Document)

- Code style analysis engine
- Complexity metrics
- AST-level checks
- CI/CD integration surface
- LSP integration
- Plugin system

---

## 9. Acceptance Criteria (Wrapper MVP)

- [x] `policycheck uv add <pkg>` runs OSV pre-install purl scan; blocks on CRITICAL/HIGH
- [x] Post-install lockfile scan runs after successful install; same severity gate
- [x] `policycheck gofumpt -l ./... -then go test ./...` blocks test if formatter exits non-zero
- [x] `policycheck run <macro>` executes all steps in order; respects `on_failure` (`stop` / `continue`)
- [x] Global config loaded from XDG path (Linux/macOS) or `%APPDATA%` (Windows)
- [x] Repo config (`policy-gate.toml`) resolved by CWD walk; merged with global config
- [x] `policycheck config init` scaffolds a valid repo `policy-gate.toml`
- [x] All blocks emit a human-readable reason and an explicit override or failure cause
- [ ] `--dry-run` on `run` prints steps without executing
- [ ] Non-TTY mode auto-blocks on MODERATE (no prompt); TTY prompts y/n
- [x] All child processes are terminated on failure before `policycheck` exits — no ghost processes under any code path
- [x] `policycheck fmt headers` injects correct path comments for Go, Python, and TypeScript files
- [x] `policycheck fmt headers --dry-run` exits 1 if any file would be modified (CI-safe)
- [x] Stale headers (path has changed) are corrected, not duplicated
- [x] Python files get shebang on line 1 and path comment on line 2; existing shebangs are preserved

Acceptance criteria intentionally deferred by the reconciliation note:
- Macro `--dry-run`
- Interactive MODERATE-risk prompt handling

---

## 10. Author Notes

**On OSV v2 syntax:** All integration code must use v2 CLI flags. The purl format (`pkg:pypi/name@version`) is the correct identifier for pre-install single-package queries. The `--format json` flag is essential for machine-readable output — do not parse human-formatted output.

**On two-phase OSV gate:** The pre-install purl check is fast and catches known direct vulnerabilities. The post-install lockfile scan is the safety net for transitive deps. Worth making the post-install scan configurable (always-on vs opt-in) since it adds latency after every install.

**On cross-platform config paths:** Use `os.UserConfigDir()` in Go — it returns the correct platform-specific config directory (`~/.config` on Linux, `%AppData%` on Windows, `~/Library/Application Support` on macOS) without any manual platform detection.

**On process cleanup:** This is the one place where Linux and Windows diverge meaningfully at the implementation level. On Linux/macOS, process groups via `Setpgid` are the right primitive — a single `kill(-pgid, SIGTERM)` reaps the whole tree. On Windows there is no pgid equivalent; Job Objects are the correct approach but add complexity. For the WSL2 development phase, the Linux path is sufficient. Flag the Windows Job Object implementation as a known gap before any native Windows release.

**On config strictness enforcement** (repo cannot lower global security thresholds): Encode this in `loader.go` at load time, not at gate execution time. A validation error on startup is impossible to bypass silently.

**On macro `rollback`:** Removed from MVP scope entirely. `stop` is the safe default. Any step with side effects that need undoing (migrations, publishes) should be handled by the tool itself, not by `policycheck` trying to invert shell commands.

---

## 11. Test Strategy

### 11.1 Approach

Strict RED / GREEN TDD. The test file is created first, before any implementation file exists. Each test describes exactly one behaviour, named as a sentence. The test must fail to compile or fail to run before any implementation is written — that is the RED phase. Only then is the minimum code written to make it pass — GREEN. No implementation is written without a failing test driving it.

**Cycle per behaviour:**
1. Write one failing test (RED — does not compile or asserts false)
2. Write minimum implementation to pass (GREEN)
3. Refactor without breaking (REFACTOR)
4. Repeat for next behaviour

**Test stack:**
- `github.com/stretchr/testify/assert` — non-fatal assertions, test continues on failure
- `github.com/stretchr/testify/require` — fatal assertions, test stops immediately on failure
- `github.com/stretchr/testify/mock` — mock expectations on port interfaces
- `github.com/vektra/mockery` — generates mocks from interfaces into `internal/mocks/`

**Hard rules:**
- One behaviour per test function — one logical `assert` or `require` call
- Test name is a sentence: `TestArgParser_UVAdd_PinnedVersion_ReturnsPypiPurl`
- Test file exists and is RED before implementation file is created
- Mocks generated, never hand-rolled: `mockery --all --dir internal/ports --output internal/mocks --outpkg mocks`
- No `t.Log` spam — silent on pass
- Integration tests tagged `//go:build integration`, excluded from default `go test ./...`
- `require` for preconditions and setup; `assert` for the actual behaviour under test

---

### 11.2 RED / GREEN Cycles by Package

The order below is the implementation order. Each package's tests are written in full before moving to the next package.

---

#### CYCLE 1 — `internal/command/argparse.go`

Write `argparse_test.go` first. The file `argparse.go` does not exist yet — all tests below are RED (will not compile).

```go
// internal/command/argparse_test.go

// RED: ParsePkgArgs does not exist yet — will not compile

func TestArgParser_UVAdd_PinnedVersion_ReturnsPypiPurl(t *testing.T) {
    got, err := ParsePkgArgs("uv", []string{"add", "fastapi==0.111.0"})
    require.NoError(t, err)
    assert.Equal(t, []string{"pkg:pypi/fastapi@0.111.0"}, got)
}

// GREEN: implement ParsePkgArgs to handle "uv" + "==" version syntax
// REFACTOR: extract version separator detection

func TestArgParser_UVAdd_UnpinnedPackage_ReturnsUnversionedPurl(t *testing.T) {
    got, err := ParsePkgArgs("uv", []string{"add", "httpx"})
    require.NoError(t, err)
    assert.Equal(t, []string{"pkg:pypi/httpx"}, got)
}

// GREEN: handle missing version — purl without @version

func TestArgParser_PipInstall_MultiplePackages_ReturnsAllPurls(t *testing.T) {
    got, err := ParsePkgArgs("pip", []string{"install", "fastapi==0.111.0", "httpx>=0.27.0"})
    require.NoError(t, err)
    assert.Len(t, got, 2)
    assert.Equal(t, "pkg:pypi/fastapi@0.111.0", got[0])
    assert.Equal(t, "pkg:pypi/httpx", got[1])  // >= is unpinned, no version in purl
}

// GREEN: handle multiple args, strip >= >= ~ operators as unpinned

func TestArgParser_GoGet_ModulePath_ReturnsGolangPurl(t *testing.T) {
    got, err := ParsePkgArgs("go", []string{"get", "github.com/some/pkg@v1.2.0"})
    require.NoError(t, err)
    assert.Equal(t, []string{"pkg:golang/github.com/some/pkg@v1.2.0"}, got)
}

// GREEN: handle go ecosystem, @ version separator

func TestArgParser_BunAdd_ScopedPackage_EncodesScope(t *testing.T) {
    got, err := ParsePkgArgs("bun", []string{"add", "@types/node@20.0.0"})
    require.NoError(t, err)
    assert.Equal(t, []string{"pkg:npm/%40types%2Fnode@20.0.0"}, got)
}

// GREEN: percent-encode scoped npm packages

func TestArgParser_UnknownManager_ReturnsError(t *testing.T) {
    _, err := ParsePkgArgs("cargo", []string{"add", "serde"})
    require.Error(t, err)
    assert.Contains(t, err.Error(), "unsupported package manager: cargo")
}

// GREEN: return typed error for unknown manager

func TestArgParser_EmptyPackageList_ReturnsError(t *testing.T) {
    _, err := ParsePkgArgs("uv", []string{"add"})
    require.Error(t, err)
    assert.Contains(t, err.Error(), "no packages specified")
}

// GREEN: guard against empty package list
```

---

#### CYCLE 2 — `internal/command/severity.go`

Write `severity_test.go` first. `severity.go` does not exist — RED.

```go
// internal/command/severity_test.go

// RED: Evaluate, GateAction, SeverityConfig do not exist yet

func TestSeverity_Critical_AlwaysBlocks(t *testing.T) {
    cfg := SeverityConfig{BlockOn: []string{"CRITICAL", "HIGH"}}
    got := Evaluate("CRITICAL", cfg, true, "")
    assert.Equal(t, GateBlock, got)
}

// GREEN: implement Evaluate, return GateBlock when severity in BlockOn

func TestSeverity_High_AlwaysBlocks(t *testing.T) {
    cfg := SeverityConfig{BlockOn: []string{"CRITICAL", "HIGH"}}
    got := Evaluate("HIGH", cfg, true, "")
    assert.Equal(t, GateBlock, got)
}

// GREEN: HIGH is in BlockOn — already passes after CRITICAL GREEN

func TestSeverity_Moderate_InTTY_Warns(t *testing.T) {
    cfg := SeverityConfig{BlockOn: []string{"CRITICAL", "HIGH"}, WarnOn: []string{"MODERATE"}}
    got := Evaluate("MODERATE", cfg, true, "")
    assert.Equal(t, GateWarn, got)
}

// GREEN: if severity in WarnOn and isTTY → GateWarn

func TestSeverity_Moderate_NonTTY_Blocks(t *testing.T) {
    cfg := SeverityConfig{BlockOn: []string{"CRITICAL", "HIGH"}, WarnOn: []string{"MODERATE"}}
    got := Evaluate("MODERATE", cfg, false, "")
    assert.Equal(t, GateBlock, got)
}

// GREEN: if severity in WarnOn and !isTTY → GateBlock

func TestSeverity_Low_AlwaysAllows(t *testing.T) {
    cfg := SeverityConfig{BlockOn: []string{"CRITICAL", "HIGH"}, WarnOn: []string{"MODERATE"}}
    got := Evaluate("LOW", cfg, false, "")
    assert.Equal(t, GateAllow, got)
}

// GREEN: not in BlockOn or WarnOn → GateAllow

func TestSeverity_AllowRiskHigh_OverridesHighBlock(t *testing.T) {
    cfg := SeverityConfig{BlockOn: []string{"CRITICAL", "HIGH"}}
    got := Evaluate("HIGH", cfg, false, "high")
    assert.Equal(t, GateAllow, got)
}

// GREEN: allowRisk == "high" lifts HIGH block

func TestSeverity_AllowRiskHigh_DoesNotLiftCritical(t *testing.T) {
    cfg := SeverityConfig{BlockOn: []string{"CRITICAL", "HIGH"}}
    got := Evaluate("CRITICAL", cfg, false, "high")
    assert.Equal(t, GateBlock, got)
}

// GREEN: allowRisk == "high" does not lift CRITICAL

func TestSeverity_AllowRiskCritical_LiftsAllBlocks(t *testing.T) {
    cfg := SeverityConfig{BlockOn: []string{"CRITICAL", "HIGH"}}
    got := Evaluate("CRITICAL", cfg, false, "critical")
    assert.Equal(t, GateAllow, got)
}

// GREEN: allowRisk == "critical" lifts CRITICAL
```

---

#### CYCLE 3 — `internal/command/chain.go`

Write `chain_test.go` first. Uses a mock `Executor` — `chain.go` and `ports/command_port.go` do not exist yet — RED.

```go
// internal/command/chain_test.go

// RED: Chain, NewChain, Executor port do not exist yet

func TestChain_GateFails_MainCommandNeverStarts(t *testing.T) {
    mockExec := mocks.NewMockExecutor(t)
    mockExec.On("Run", "gofumpt", []string{"-l", "./..."}).Return(1, nil)

    chain := NewChain(mockExec)
    err := chain.Execute("gofumpt -l ./...", "go test ./...")

    require.Error(t, err)
    mockExec.AssertNotCalled(t, "Run", "go", mock.Anything)
}

// GREEN: implement Chain.Execute — if gate exit != 0, return error before running main

func TestChain_GatePasses_MainCommandRuns(t *testing.T) {
    mockExec := mocks.NewMockExecutor(t)
    mockExec.On("Run", "gofumpt", []string{"-l", "./..."}).Return(0, nil)
    mockExec.On("Run", "go", []string{"test", "./..."}).Return(0, nil)

    chain := NewChain(mockExec)
    err := chain.Execute("gofumpt -l ./...", "go test ./...")

    require.NoError(t, err)
    mockExec.AssertExpectations(t)
}

// GREEN: gate exit 0 → run main command

func TestChain_GateFails_KillGroupCalled(t *testing.T) {
    mockExec := mocks.NewMockExecutor(t)
    mockExec.On("Run", "gofumpt", []string{"-l", "./..."}).Return(1, nil)
    mockExec.On("KillGroup").Return(nil).Once()

    chain := NewChain(mockExec)
    _ = chain.Execute("gofumpt -l ./...", "go test ./...")

    mockExec.AssertCalled(t, "KillGroup")
}

// GREEN: on gate failure, call KillGroup before returning

func TestChain_MainFails_KillGroupCalled(t *testing.T) {
    mockExec := mocks.NewMockExecutor(t)
    mockExec.On("Run", "gofumpt", []string{"-l", "./..."}).Return(0, nil)
    mockExec.On("Run", "go", []string{"test", "./..."}).Return(1, nil)
    mockExec.On("KillGroup").Return(nil).Once()

    chain := NewChain(mockExec)
    _ = chain.Execute("gofumpt -l ./...", "go test ./...")

    mockExec.AssertCalled(t, "KillGroup")
}

// GREEN: on main command failure, also call KillGroup — no ghost processes
```

---

#### CYCLE 4 — `internal/adapters/command/detector.go`

Write `detector_test.go` first. `detector.go` does not exist — RED.

```go
// internal/adapters/command/detector_test.go

// RED: Detector, Classify, CommandType constants do not exist yet

func TestDetector_UVAdd_ClassifiedAsPackageGate(t *testing.T) {
    d := NewDetector(&config.Config{})
    got := d.Classify([]string{"uv", "add", "fastapi"})
    assert.Equal(t, PackageGate, got)
}

// GREEN: implement Classify — check args[0] against known pkg manager names

func TestDetector_GoGet_ClassifiedAsPackageGate(t *testing.T) {
    d := NewDetector(&config.Config{})
    got := d.Classify([]string{"go", "get", "github.com/x/y"})
    assert.Equal(t, PackageGate, got)
}

// GREEN: "go" with "get" subcommand is a package gate — not all "go" invocations are

func TestDetector_GoTest_WithoutThen_ClassifiedAsPassthrough(t *testing.T) {
    d := NewDetector(&config.Config{})
    got := d.Classify([]string{"go", "test", "./..."})
    assert.Equal(t, Passthrough, got)
}

// GREEN: "go test" without -then is passthrough — go get is the only pkg gate for go

func TestDetector_ArgContainsThen_ClassifiedAsToolingGate(t *testing.T) {
    d := NewDetector(&config.Config{})
    got := d.Classify([]string{"gofumpt", "-l", "./...", "-then", "go", "test", "./..."})
    assert.Equal(t, ToolingGate, got)
}

// GREEN: presence of "-then" in args → ToolingGate

func TestDetector_KnownMacroName_ClassifiedAsMacroRunner(t *testing.T) {
    cfg := &config.Config{Macros: map[string]config.Macro{"commit-push": {}}}
    d := NewDetector(cfg)
    got := d.Classify([]string{"run", "commit-push"})
    assert.Equal(t, MacroRunner, got)
}

// GREEN: "run" + known macro name → MacroRunner

func TestDetector_UnknownSubcommand_ClassifiedAsPassthrough(t *testing.T) {
    d := NewDetector(&config.Config{})
    got := d.Classify([]string{"git", "status"})
    assert.Equal(t, Passthrough, got)
}

// GREEN: nothing matched → Passthrough

func TestDetector_EmptyArgs_ClassifiedAsPassthrough(t *testing.T) {
    d := NewDetector(&config.Config{})
    got := d.Classify([]string{})
    assert.Equal(t, Passthrough, got)
}

// GREEN: guard against empty args — no panic
```

---

#### CYCLE 5 — `internal/adapters/macro/runner.go`

Write `runner_test.go` first. Uses mock `Executor`. `runner.go` does not exist — RED.

```go
// internal/adapters/macro/runner_test.go

// RED: Runner, NewRunner do not exist yet

func TestMacroRunner_SingleStep_Success_NoError(t *testing.T) {
    mockExec := mocks.NewMockExecutor(t)
    mockExec.On("Run", "git", []string{"add", "-A"}).Return(0, nil)

    macro := config.Macro{Steps: []string{"git add -A"}, OnFailure: "stop"}
    runner := NewRunner(mockExec)
    err := runner.Run(macro, map[string]string{})

    require.NoError(t, err)
    mockExec.AssertExpectations(t)
}

// GREEN: implement Run — execute each step, return nil on all success

func TestMacroRunner_StopOnFailure_SecondStepNeverRuns(t *testing.T) {
    mockExec := mocks.NewMockExecutor(t)
    mockExec.On("Run", "git", []string{"add", "-A"}).Return(1, nil)
    mockExec.On("KillGroup").Return(nil)

    macro := config.Macro{
        Steps:     []string{"git add -A", "git push origin main"},
        OnFailure: "stop",
    }
    runner := NewRunner(mockExec)
    err := runner.Run(macro, map[string]string{})

    require.Error(t, err)
    mockExec.AssertNotCalled(t, "Run", "git", []string{"push", "origin", "main"})
}

// GREEN: on_failure=stop → halt at first failure, do not run subsequent steps

func TestMacroRunner_StopOnFailure_KillGroupCalled(t *testing.T) {
    mockExec := mocks.NewMockExecutor(t)
    mockExec.On("Run", "git", []string{"add", "-A"}).Return(1, nil)
    mockExec.On("KillGroup").Return(nil).Once()

    macro := config.Macro{Steps: []string{"git add -A"}, OnFailure: "stop"}
    runner := NewRunner(mockExec)
    _ = runner.Run(macro, map[string]string{})

    mockExec.AssertCalled(t, "KillGroup")
}

// GREEN: KillGroup always called on step failure — no ghost processes

func TestMacroRunner_ContinueOnFailure_AllStepsAttempted(t *testing.T) {
    mockExec := mocks.NewMockExecutor(t)
    mockExec.On("Run", "ruff", mock.Anything).Return(1, nil)
    mockExec.On("Run", "mypy", mock.Anything).Return(0, nil)
    mockExec.On("Run", "pytest", mock.Anything).Return(0, nil)
    mockExec.On("KillGroup").Return(nil)

    macro := config.Macro{
        Steps:     []string{"ruff format .", "mypy src/", "pytest"},
        OnFailure: "continue",
    }
    runner := NewRunner(mockExec)
    err := runner.Run(macro, map[string]string{})

    require.Error(t, err) // aggregate error — ruff failed
    mockExec.AssertCalled(t, "Run", "mypy", mock.Anything)
    mockExec.AssertCalled(t, "Run", "pytest", mock.Anything)
}

// GREEN: on_failure=continue → run all steps, return aggregate error

func TestMacroRunner_TemplateVar_SubstitutedBeforeExec(t *testing.T) {
    mockExec := mocks.NewMockExecutor(t)
    mockExec.On("Run", "git", []string{"commit", "-m", "fix: typo"}).Return(0, nil)

    macro := config.Macro{
        Steps:     []string{`git commit -m "{{.message}}"`},
        OnFailure: "stop",
    }
    runner := NewRunner(mockExec)
    err := runner.Run(macro, map[string]string{"message": "fix: typo"})

    require.NoError(t, err)
    mockExec.AssertExpectations(t)
}

// GREEN: {{.var}} replaced with supplied vars before exec

func TestMacroRunner_TemplateVar_Missing_ReturnsError(t *testing.T) {
    mockExec := mocks.NewMockExecutor(t)

    macro := config.Macro{
        Steps:     []string{`git commit -m "{{.message}}"`},
        OnFailure: "stop",
    }
    runner := NewRunner(mockExec)
    err := runner.Run(macro, map[string]string{}) // message not supplied

    require.Error(t, err)
    assert.Contains(t, err.Error(), "missing template variable: message")
    mockExec.AssertNotCalled(t, "Run", mock.Anything, mock.Anything)
}

// GREEN: missing var → error before any step runs
```

---

#### CYCLE 6 — `internal/config/loader.go`

Write `loader_test.go` first. Uses `t.TempDir()` — no real config files. `loader.go` does not exist — RED.

```go
// internal/config/loader_test.go

// RED: Loader, NewLoader, Load do not exist yet

func TestLoader_GlobalConfigOnly_LoadsSuccessfully(t *testing.T) {
    dir := t.TempDir()
    writeToml(t, dir, "global.toml", `
        [security]
        block_on = ["CRITICAL", "HIGH"]
        warn_on  = ["MODERATE"]
    `)

    loader := NewLoader(filepath.Join(dir, "global.toml"), dir)
    cfg, err := loader.Load()

    require.NoError(t, err)
    assert.Equal(t, []string{"CRITICAL", "HIGH"}, cfg.Security.BlockOn)
}

// GREEN: implement Load — parse global TOML into Config struct

func TestLoader_RepoConfig_OverridesGlobalScalars(t *testing.T) {
    dir := t.TempDir()
    writeToml(t, dir, "global.toml", `
        [security]
        block_on = ["CRITICAL", "HIGH"]
    `)
    writeToml(t, dir, "policy-gate.toml", `
        [security]
        block_on = ["CRITICAL", "HIGH", "MODERATE"]
    `)

    loader := NewLoader(filepath.Join(dir, "global.toml"), dir)
    cfg, err := loader.Load()

    require.NoError(t, err)
    assert.Equal(t, []string{"CRITICAL", "HIGH", "MODERATE"}, cfg.Security.BlockOn)
}

// GREEN: merge repo config over global — repo scalar wins

func TestLoader_RepoConfig_CannotLowerGlobalBlockThreshold(t *testing.T) {
    dir := t.TempDir()
    writeToml(t, dir, "global.toml", `
        [security]
        block_on = ["CRITICAL", "HIGH"]
    `)
    writeToml(t, dir, "policy-gate.toml", `
        [security]
        block_on = ["CRITICAL"]
    `)

    loader := NewLoader(filepath.Join(dir, "global.toml"), dir)
    _, err := loader.Load()

    require.Error(t, err)
    assert.Contains(t, err.Error(), "repo config cannot lower global security threshold")
}

// GREEN: validate merge — repo block_on must be superset of global block_on

func TestLoader_MalformedGlobalToml_ReturnsError(t *testing.T) {
    dir := t.TempDir()
    writeRaw(t, dir, "global.toml", `this is not toml :::`)

    loader := NewLoader(filepath.Join(dir, "global.toml"), dir)
    _, err := loader.Load()

    require.Error(t, err)
    assert.Contains(t, err.Error(), "failed to parse global config")
}

// GREEN: surface parse error with clear message

func TestLoader_MacrosFromBothConfigs_AreMerged(t *testing.T) {
    dir := t.TempDir()
    writeToml(t, dir, "global.toml", `
        [macros.global-macro]
        steps = ["echo global"]
        on_failure = "stop"
    `)
    writeToml(t, dir, "policy-gate.toml", `
        [macros.repo-macro]
        steps = ["echo repo"]
        on_failure = "stop"
    `)

    loader := NewLoader(filepath.Join(dir, "global.toml"), dir)
    cfg, err := loader.Load()

    require.NoError(t, err)
    assert.Contains(t, cfg.Macros, "global-macro")
    assert.Contains(t, cfg.Macros, "repo-macro")
}

// GREEN: macros merged by key — both global and repo macros available
```

---

#### CYCLE 7 — `internal/config/resolver.go`

Write `resolver_test.go` first. `resolver.go` does not exist — RED.

```go
// internal/config/resolver_test.go

// RED: Resolver, NewResolver, Resolve do not exist yet

func TestResolver_FindsPolicyGateInCWD(t *testing.T) {
    dir := t.TempDir()
    writeRaw(t, dir, "policy-gate.toml", "")

    resolver := NewResolver()
    got, err := resolver.Resolve(dir)

    require.NoError(t, err)
    assert.Equal(t, dir, got)
}

// GREEN: implement Resolve — check CWD for policy-gate.toml first

func TestResolver_FindsPolicyGateInParentDirectory(t *testing.T) {
    root := t.TempDir()
    sub := filepath.Join(root, "a", "b", "c")
    require.NoError(t, os.MkdirAll(sub, 0755))
    writeRaw(t, root, "policy-gate.toml", "")

    resolver := NewResolver()
    got, err := resolver.Resolve(sub)

    require.NoError(t, err)
    assert.Equal(t, root, got)
}

// GREEN: walk upward until policy-gate.toml found

func TestResolver_NoPolicyGateAnywhere_ReturnsEmptyString(t *testing.T) {
    dir := t.TempDir() // no policy-gate.toml

    resolver := NewResolver()
    got, err := resolver.Resolve(dir)

    require.NoError(t, err)
    assert.Empty(t, got)
}

// GREEN: return "" when not found — not an error, global config only applies

func TestResolver_StopsAtFilesystemRoot(t *testing.T) {
    // walk must not loop forever or panic at /
    resolver := NewResolver()
    _, err := resolver.Resolve("/")

    require.NoError(t, err)
}

// GREEN: guard against infinite walk — stop at fs root
```

---

#### CYCLE 8 — `internal/fmt/header.go`

Write `header_test.go` first. `header.go` does not exist — RED.

```go
// internal/fmt/header_test.go

// RED: HasHeader, InjectHeader do not exist yet

func TestHeader_GoFile_WithoutHeader_ReturnsFalse(t *testing.T) {
    content := "package main\n\nfunc main() {}\n"
    assert.False(t, HasHeader(content, "go", "internal/main.go"))
}

// GREEN: implement HasHeader — scan first line for // <path>

func TestHeader_GoFile_WithCorrectHeader_ReturnsTrue(t *testing.T) {
    content := "// internal/main.go\npackage main\n"
    assert.True(t, HasHeader(content, "go", "internal/main.go"))
}

// GREEN: exact match on first line → true

func TestHeader_GoFile_WithStaleHeader_ReturnsFalse(t *testing.T) {
    content := "// internal/old/main.go\npackage main\n"
    assert.False(t, HasHeader(content, "go", "internal/main.go"))
}

// GREEN: header present but path is wrong → false (stale)

func TestHeader_GoFile_MissingHeader_InjectsOnLineOne(t *testing.T) {
    content := "package main\n"
    got := InjectHeader(content, "go", "internal/main.go")
    assert.Equal(t, "// internal/main.go\npackage main\n", got)
}

// GREEN: implement InjectHeader — prepend comment line

func TestHeader_GoFile_StaleHeader_ReplacesNotDuplicates(t *testing.T) {
    content := "// internal/old/main.go\npackage main\n"
    got := InjectHeader(content, "go", "internal/main.go")
    assert.Equal(t, "// internal/main.go\npackage main\n", got)
    assert.Equal(t, 2, strings.Count(got, "\n"))
}

// GREEN: stale header replaced on line 1 — not prepended (would duplicate)

func TestHeader_PythonFile_NoShebang_InjectsShebangAndPath(t *testing.T) {
    content := "import os\n"
    got := InjectHeader(content, "python", "scripts/run.py")
    assert.Equal(t, "#!/usr/bin/env python3\n# scripts/run.py\nimport os\n", got)
}

// GREEN: Python without shebang → add both shebang and path comment

func TestHeader_PythonFile_ExistingShebang_PreservesItAndInjectsPath(t *testing.T) {
    content := "#!/usr/bin/env python3\nimport os\n"
    got := InjectHeader(content, "python", "scripts/run.py")
    assert.Equal(t, "#!/usr/bin/env python3\n# scripts/run.py\nimport os\n", got)
}

// GREEN: shebang already present → keep it, insert path comment on line 2

func TestHeader_PythonFile_StalePath_CorrectesLine2(t *testing.T) {
    content := "#!/usr/bin/env python3\n# scripts/old.py\nimport os\n"
    got := InjectHeader(content, "python", "scripts/run.py")
    assert.Equal(t, "#!/usr/bin/env python3\n# scripts/run.py\nimport os\n", got)
}

// GREEN: stale path comment on line 2 replaced — shebang untouched

func TestHeader_TypeScriptFile_MissingHeader_InjectsOnLineOne(t *testing.T) {
    content := "export const x = 1\n"
    got := InjectHeader(content, "typescript", "src/utils/x.ts")
    assert.Equal(t, "// src/utils/x.ts\nexport const x = 1\n", got)
}

// GREEN: TypeScript same as Go — single comment line 1
```

---

#### CYCLE 9 — `internal/fmt/walker.go`

Write `walker_test.go` first. `walker.go` does not exist — RED.

```go
// internal/fmt/walker_test.go

// RED: Walker, NewWalker, Collect, Run do not exist yet

func TestWalker_CollectsGoFiles(t *testing.T) {
    root := t.TempDir()
    createFile(t, root, "main.go", "package main\n")

    walker := NewWalker(root, nil)
    files, err := walker.Collect()

    require.NoError(t, err)
    assert.Contains(t, relPaths(files, root), "main.go")
}

// GREEN: implement Collect — walk root, return .go files

func TestWalker_SkipsVendorDirectory(t *testing.T) {
    root := t.TempDir()
    createFile(t, root, "vendor/lib/lib.go", "package lib\n")
    createFile(t, root, "internal/main.go", "package main\n")

    walker := NewWalker(root, nil)
    files, err := walker.Collect()

    require.NoError(t, err)
    paths := relPaths(files, root)
    assert.NotContains(t, paths, "vendor/lib/lib.go")
    assert.Contains(t, paths, "internal/main.go")
}

// GREEN: skip entries whose path contains a SKIP_DIRS entry

func TestWalker_SkipsNodeModules(t *testing.T) {
    root := t.TempDir()
    createFile(t, root, "node_modules/pkg/index.ts", "")
    createFile(t, root, "src/index.ts", "")

    walker := NewWalker(root, nil)
    files, err := walker.Collect()

    require.NoError(t, err)
    assert.NotContains(t, relPaths(files, root), "node_modules/pkg/index.ts")
}

// GREEN: node_modules in SKIP_DIRS

func TestWalker_ExtensionFilter_OnlyReturnsMatchingFiles(t *testing.T) {
    root := t.TempDir()
    createFile(t, root, "main.go", "")
    createFile(t, root, "main.py", "")
    createFile(t, root, "main.ts", "")

    walker := NewWalker(root, []string{".go"})
    files, err := walker.Collect()

    require.NoError(t, err)
    paths := relPaths(files, root)
    assert.Contains(t, paths, "main.go")
    assert.NotContains(t, paths, "main.py")
    assert.NotContains(t, paths, "main.ts")
}

// GREEN: filter by extension when non-nil filter supplied

func TestWalker_DryRun_ReportsModifiedCount_WritesNothing(t *testing.T) {
    root := t.TempDir()
    createFile(t, root, "main.go", "package main\n") // no header

    walker := NewWalker(root, nil)
    result, err := walker.Run(true)

    require.NoError(t, err)
    assert.Equal(t, 1, result.Modified)

    raw, _ := os.ReadFile(filepath.Join(root, "main.go"))
    assert.Equal(t, "package main\n", string(raw)) // unchanged
}

// GREEN: dry-run=true → count modified but do not write

func TestWalker_Run_WritesHeaderToFile(t *testing.T) {
    root := t.TempDir()
    createFile(t, root, "main.go", "package main\n")

    walker := NewWalker(root, nil)
    result, err := walker.Run(false)

    require.NoError(t, err)
    assert.Equal(t, 1, result.Modified)

    raw, _ := os.ReadFile(filepath.Join(root, "main.go"))
    assert.Equal(t, "// main.go\npackage main\n", string(raw))
}

// GREEN: dry-run=false → write header to file on disk

func TestWalker_Run_Idempotent_SecondRunModifiesNothing(t *testing.T) {
    root := t.TempDir()
    createFile(t, root, "main.go", "package main\n")

    walker := NewWalker(root, nil)
    _, _ = walker.Run(false)         // first run — adds header
    result, err := walker.Run(false) // second run — nothing to do

    require.NoError(t, err)
    assert.Equal(t, 0, result.Modified)
}

// GREEN: HasHeader returns true after first run → file skipped on second run
```

---

### 11.3 Mock Inventory (`internal/mocks/`)

Generated from port interfaces. Never hand-rolled.

| Mock file                  | Source interface        | Used in cycles         |
| -------------------------- | ----------------------- | ---------------------- |
| `mock_executor.go`         | `ports.Executor`        | 3, 5                   |
| `mock_security_gate.go`    | `ports.SecurityGate`    | adapter security tests |
| `mock_macro_runner.go`     | `ports.MacroRunner`     | router tests           |
| `mock_config_loader.go`    | `ports.ConfigLoader`    | router tests           |
| `mock_header_formatter.go` | `ports.HeaderFormatter` | router tests           |

Regenerate after any port interface change:
```bash
mockery --all --dir internal/ports --output internal/mocks --outpkg mocks
```

---

### 11.4 Integration Tests

Tagged `//go:build integration`. Run with:
```bash
go test -tags=integration ./...
```

Require `osv-scanner` or `osv-scanner.exe` in PATH.

| Test                                             | Cycle   | What it proves                                                        |
| ------------------------------------------------ | ------- | --------------------------------------------------------------------- |
| `TestIntegration_OSVCLIBinary_ScansRealPurl`     | after 2 | Real subprocess returns parseable JSON for known vulnerable package   |
| `TestIntegration_Chain_RealGofumpt_BlocksGoTest` | after 3 | Real `gofumpt` gate blocks `go test` on deliberately unformatted file |
| `TestIntegration_MacroRunner_RealShellSteps`     | after 5 | Multi-step macro runs real commands; `stop` halts on first failure    |
| `TestIntegration_ConfigResolver_RealNestedFS`    | after 7 | Resolver finds `policy-gate.toml` walking up a real directory tree    |
| `TestIntegration_FmtHeaders_RealRepo_Idempotent` | after 9 | Headers written correctly; second run modifies zero files             |

---
