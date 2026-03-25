# policycheck — Rewrite Design Document

**Version 1.1 · March 2026**

---

## Table of Contents

1. [Overview](#1-overview)
2. [Problems With the Current Design](#2-problems-with-the-current-design)
3. [Target Design](#3-target-design)
4. [Coding Standards](#4-coding-standards)
5. [Full Config Struct Map](#5-full-config-struct-map)
6. [Test Strategy](#6-test-strategy)
7. [Migration Path](#7-migration-path)
8. [What Does Not Change](#8-what-does-not-change)

---

## 1. Overview

policycheck is a local developer policy validator for Go repositories. The current implementation is a ~4,000-line single-module codebase that accumulated features over time without a deliberate structural design. It works, but the seams between policies have blurred, several policies have hardcoded behaviour that bypasses configuration, and the lack of clean injection points makes tests require real filesystem construction for almost every assertion.

This document defines the target design for a rewrite that:

- Preserves all existing policy behaviour exactly
- Closes all configurability gaps identified in Section 2.2
- Produces a package structure where each file and group owns one concern
- Expands the pure-function surface so most logic can be tested without touching disk
- Removes the six independent copies of the same directory-walk pattern
- Adds a custom rules group for user-defined code smell and security patterns
- Uses the router as the single host wiring surface for config, walk, and scanner providers

The rewrite is **reference-based, not from scratch**. Every check, threshold, and config field in the current code is the reference implementation. Nothing is removed; everything is rearchitected.

The binary must pass its own policy checks when run against its own source. That is the primary acceptance criterion for the rewrite.

---

## 2. Problems With the Current Design

### 2.1 File Organisation Does Not Match Concerns

The most visible problem is `security.go`, which contains three unrelated checks — secret logging, test file location, and CLI formatter — plus the external scanner subprocess plumbing. The file name implies one concern but delivers four.

| Current file | Actual contents |
|---|---|
| `core/security.go` | Secret logging + test location + CLI formatter + external scanner subprocess plumbing |
| `core/hygiene.go` | Symbol naming + doc comments (related, co-location is appropriate) |
| `core/contracts.go` | AI compatibility + scope guard + contract resolution helpers |
| `core/topology.go` | Package file count + doc.go presence/structure (related, fine) |
| `core/quality.go` | File size + Go function quality + external scanner dispatch |
| `core/warnings.go` | Warning compression/summarisation only |

The practical consequence is that test files have no obvious homes, and navigating to a specific policy check requires knowing which unrelated file it lives in.

### 2.2 Hardcoded Behaviour That Bypasses Config

Six policies have logic that cannot be controlled from `policy-gate.toml`. Adapting them requires editing source code:

| Policy | Hardcoded element | Impact |
|---|---|---|
| 1 – Go Version | Allowed prefixes `"1.24"`, `"1.25"` | Adopting Go 1.26 requires a source edit |
| 7 – Symbol Names | Scan roots (`internal/`, `cmd/`) and minimum token count (2) | Repos with `src/` or `app/` roots are silently skipped |
| 8 – Doc Style | Same hardcoded scan roots as Policy 7 | Same silent skip problem |
| 9 – AI Compatibility | Requires both `--ai` AND `--user` in target file | Cannot require only `--ai` |
| 10 – Scope Guard | Constant name `"ScopeProjectRepo"` and forbidden calls | Cannot adapt to a different scope name |
| 11 – Package Rules | Max 10 files, scan roots, min/max concern count | Thresholds not adjustable per repo |

Additionally, the secret logging suppression logic — the benign-hint list (`"example"`, `"sample"`, `"fixture"`, etc.) and the placeholder strings (`"<token>"`, `"changeme"`, etc.) — is entirely hardcoded and cannot be extended or disabled from config.

### 2.3 No Pure-Function Injection Points for Checks

All `Check*` entry points open files themselves. There is no way to pass pre-read content or a pre-parsed AST. Every test must construct a real temporary directory tree, even for a test that is logically about a single string-matching rule.

The subset of code that is already pure and testable without filesystem access:

- `evaluateFunctionQualityFacts`, `BuildFunctionQualityWarning`
- `appendFunctionWarnings`, `partitionFunctionWarnings`, `appendSummarizedMildWarnings`
- `parseDocGoConcerns`, `hasMultipleTokens`, `hasDocWithPrefix`
- `hasAudienceModeSupport`, `isBenignSecretExample`, `isObviousPlaceholderSecret`, `hasSecretKeyword`
- `pickBestSecretFinding`, `filterAllowlistedSecretFindings`, `secretSeverityRank`
- `ValidatePolicyConfig`, `ApplyPolicyConfigDefaults`, `parsePolicyFactsOutput`

Everything else — the `Check*` orchestration layer — requires filesystem access. The rewrite expands the pure-function surface by applying the two-layer pattern (Section 3.4) consistently across all policies.

### 2.4 Walk Pattern Repeated Six Times

The pattern:

```
filepath.WalkDir → skip dirs → skip non-.go → check ignore prefixes → process file
```

appears independently in `security.go`, `hygiene.go`, `topology.go`, `quality.go`, and `architecture.go`. Each copy has minor variations but the skeleton is identical. A shared utility removes this duplication and gives tests a single, testable walk implementation.

### 2.5 Recalibrated Complexity Thresholds

The current function quality thresholds are stricter than SonarQube's own defaults, causing clean guard-clause functions to emit warnings. The thresholds are recalibrated as part of the rewrite:

| Band | Current | Rewrite | Rationale |
|---|---|---|---|
| Mild warn CTX | 10 | 12 | Stops flagging clean guard-clause functions |
| Elevated warn CTX | 12 | 14 | Functions worth a second look |
| Immediate refactor CTX | 14 | 16 | Genuinely complex, schedule refactor |
| Error CTX | 15 | 18 | Hard limit, blocks merge |
| Combined (ctx + loc) | ctx≥8, loc≥80 | ctx≥10, loc≥80 | Less noise on medium-complexity functions |

---

## 3. Target Design

### 3.1 Package Structure

```
cmd/policycheck/
└── main.go                 – thin entry point; boots router once, resolves host startup flow

internal/policycheck/
│
├── cli/
│   ├── doc.go
│   ├── errors.go           – error formatting and exit code determination
│   ├── warnings.go         – warning output formatting
│   └── rules.go            – flag parsing, config loading, result output
│
├── config/
│   ├── doc.go
│   ├── config_manager.go   – ApplyDefaults, Validate, cross-field checks
│   └── config_loader.go    – decode/compile raw config source supplied by the config port
│
├── core/
│   ├── doc.go
│   ├── policy_manager.go   – RunPolicyChecks orchestrator; resolves required ports via small host seam
│   ├── policy_registry.go  – policy group registration and dispatch
│   │
│   ├── contracts/          – Group 1: version, CLI formatter, AI compat, scope guard
│   │   ├── doc.go
│   │   ├── go_version.go
│   │   ├── cli_formatter.go
│   │   ├── ai_compatibility.go
│   │   └── scope_guard.go
│   │
│   ├── quality/            – Group 2: file size, function quality
│   │   ├── doc.go
│   │   ├── file_size.go
│   │   └── func_quality.go
│   │
│   ├── security/           – Group 3: secret logging
│   │   ├── doc.go
│   │   ├── secret_scan.go
│   │   └── secret_catalog.go
│   │
│   ├── hygiene/            – Group 4: symbol names, doc style
│   │   ├── doc.go
│   │   ├── symbol_names.go
│   │   └── doc_style.go
│   │
│   ├── structure/          – Group 5: test location, package rules, architecture
│   │   ├── doc.go
│   │   ├── test_location.go
│   │   ├── package_rules.go
│   │   └── architecture.go
│   │
│   └── custom/             – Group 6: user-defined regex rules
│       ├── doc.go
│       └── custom_rules.go
│
├── host/
│   ├── doc.go
│   ├── ports.go            – small provider-resolution seam used by policy_manager and config loader
│   └── bootstrap.go        – router boot helper for the policycheck command
│
├── types/                  – shared types (unchanged)
└── utils/                  – path helpers (unchanged)

internal/ports/
├── config.go               – ConfigProvider contract
├── walk.go                 – WalkProvider contract
└── scanners.go             – ScannerProvider contract

internal/adapters/
├── config/                 – concrete config provider
├── walk/                   – concrete walk provider
└── scanners/               – concrete scanner provider

internal/router/
├── ports.go                – PortConfig, PortWalk, PortScanner
├── registry_imports.go     – whitelist validation + atomic registry declaration
├── extensions.go           – host wiring for policycheck providers
├── optional_extensions.go  – optional layer
├── extension.go            – frozen router contracts
└── registry.go             – frozen router resolution/publication
```

The rewrite keeps the policy engine itself under `internal/policycheck/`, but it no
longer treats config loading, directory walking, or external scanner execution as
direct package dependencies. Those become host capabilities provided through the
router, with contracts declared in `internal/ports/`.

### 3.2 Policy Groups

| Group | Package | Policies |
|---|---|---|
| 1 – Contracts | `core/contracts/` | Go version, CLI formatter, AI compatibility, Scope guard |
| 2 – Quality | `core/quality/` | File size, Function quality |
| 3 – Security | `core/security/` | Secret logging |
| 4 – Hygiene | `core/hygiene/` | Symbol names, Doc style |
| 5 – Structure | `core/structure/` | Test location, Package rules, Architecture roots |
| 6 – Custom | `core/custom/` | User-defined regex rules |

**Group 1 — Contracts** holds checks that ask "does this repo honour a specific contract?" — version contracts, output contracts, interface contracts. All are thin file reads plus string or pattern matching with no shared state between them.

**Group 2 — Quality** holds the two checks that deal with LOC/CTX thresholds and consume `PolicyFact` output from the scanner layer. File size and function quality are kept together because the CTX penalty system in file size directly mirrors the CTX band system in function quality.

**Group 3 — Security** stands alone. The pattern catalog, severity ranking, allowlist, override system, and benign-hint suppression are complex enough that mixing them with other concerns would hurt both readability and testability.

**Group 4 — Hygiene** holds the two checks that walk Go AST declarations. They share the same file walker, the same AST parser call, and operate on the same nodes. Co-location is appropriate.

**Group 5 — Structure** holds the three checks that are about where things live in the repo rather than what is inside them. Test location is a path prefix check, package rules count files per directory, architecture roots validate directory children. None parse file content.

**Group 6 — Custom** holds user-defined regex rules loaded entirely from config. The implementation is a loop — no AST, no subprocess, no catalog. It is the escape hatch for repo-specific patterns that do not warrant a new built-in policy.

### 3.3 The Walk Package

A new router-backed walk capability replaces the six independent copies of the
directory-walk pattern. The traversal logic still has one implementation, but it
is exposed to the policy engine through the `WalkProvider` port rather than a
direct package import.

```go
package ports

type WalkProvider interface {
    WalkDirectoryTree(root string, walkFn fs.WalkDirFunc) error
}
```

The concrete stdlib implementation lives in `internal/adapters/walk/`. The
policy engine may still define internal helpers such as `GoFiles` or
`ProductionGoFiles`, but they should be thin wrappers that call the resolved
`WalkProvider` instead of owning filesystem traversal directly.

### 3.4 Check Architecture: Two Layers

Every policy check is implemented in two layers:

| Layer | Responsibility |
|---|---|
| **Pure logic functions** | Accept already-read content (string, `[]byte`, AST node, `[]PolicyFact`, etc.) and return `[]Violation` or a decision. No filesystem access. Fully unit-testable with inline inputs. |
| **Orchestration functions (`Check*`)** | Use resolved host providers to enumerate files or launch scanners, then call pure logic functions. Integration-tested with `t.TempDir()`. |

**Policy 1 — Go Version:**

```go
// Pure — testable with any string
func validateGoVersion(content string, allowedPrefixes []string) []types.Violation

// Orchestrator — gets raw config source via ConfigProvider, then calls pure function
func CheckGoVersion(root string, cfg config.PolicyConfig) []types.Violation
```

**Policy 5 — File Size (threshold math isolated):**

```go
// Pure — testable with just integers, no files needed
func computeFileSizeThresholds(cfg config.PolicyFileSizeConfig, ctxFuncCount int) (warnLOC, maxLOC int)

// Pure — testable with a line count and pre-computed thresholds
func evaluateFileSize(rel string, lineCount, warnLOC, maxLOC int) []types.Violation

// Orchestrator — file enumeration goes through WalkProvider
func CheckFileSizePolicies(root string, cfg config.PolicyConfig) []types.Violation
```

The threshold computation for Policy 5 is the most complex logic in the entire codebase and is where off-by-one errors are most likely. Isolating it as a pure function enables exhaustive table-driven tests against every boundary without any file I/O.

### 3.5 Scanner Package

The subprocess plumbing moves from `core/security.go` and `core/quality.go`
behind the `ScannerProvider` port. The concrete scanner adapter owns the full
lifecycle of external scanner invocation:

- Command detection (`python3`/`python`/`node` availability)
- Script materialisation
- Subprocess execution with timeout
- NDJSON output parsing into `[]PolicyFact`
- Worker mode management

`core/quality/func_quality.go` receives `[]PolicyFact` from a small host seam and
has no knowledge of subprocesses, runtimes, or script paths.

### 3.5.1 Router Integration

The rewrite must preserve a strict separation between policy logic and host
wiring:

- `cmd/policycheck/main.go` boots the router exactly once via `RouterBootExtensions(ctx)`.
- `internal/router/extensions.go` owns the application wiring for `PortConfig`,
  `PortWalk`, and `PortScanner`.
- `internal/ports/` declares the contracts consumed by the policy engine.
- `internal/adapters/` provides the concrete implementations.
- `internal/policycheck/` must not import `internal/router/` directly except for a
  tiny host-resolution seam if needed at the package boundary.

The important constraint is import direction: adapters may implement host
capabilities, but router wiring stays in `internal/router/`. Do not design the
rewrite around adapter packages importing `internal/router` to self-register;
that creates an import cycle once `internal/router/extensions.go` imports those
adapters for wiring.

The usage rule from `docs/router/usage.md` is the implementation baseline:

```text
consumer -> internal/ports + internal/router
host boot -> internal/router/ext
internal/router/ext -> internal/adapters/*
```

This matters because the rewrite introduces multiple host capabilities that are
tempting to couple directly. `internal/adapters/config`, `internal/adapters/walk`,
and `internal/adapters/scanners` must not import each other to reach another
capability. If one layer needs another capability, it should depend on the port
contract and resolve the provider through the router boundary rather than taking
a direct adapter dependency.

### 3.5.2 Router Boot Semantics

The rewrite must treat router boot as a one-time host startup action with the
same semantics as the router design document:

- `main.go` boots the router exactly once before config load or policy execution
- a successful boot publishes the immutable provider snapshot once
- a second boot attempt after publication returns `MultipleInitializations`
- boot publication uses `registry.CompareAndSwap(nil, &localMap)`
- if two goroutines race to boot, exactly one CAS succeeds and the loser returns
  `MultipleInitializations`
- no separate mutex or `sync.Once` is introduced for router publication

This matters for the rewrite because `cmd/policycheck/main.go` must remain the
single authoritative startup path. The rewritten engine must not introduce a
second hidden bootstrap path inside `internal/policycheck/`.

### 3.5.3 Router Guardrails

The rewrite must also preserve the router's development guardrails as explicit
host constraints, not just implementation details:

| Mechanism | Purpose | Effect on rewrite |
|---|---|---|
| Frozen/mutable split | Protect router core contracts | Rewrite work must keep host changes in mutable router files and avoid redesigning frozen router contracts casually |
| `router.lock` checksums | Detect frozen drift | Rewrite plans must assume lock integrity is checked by host tooling before broader policy analysis |
| Data-only wiring files | Limit mutation surface | Provider additions belong in `ports.go`, `registry_imports.go`, `extensions.go`, and `optional_extensions.go`, not in new ad hoc boot logic |
| Explicit error catalog | Guide diagnosis | Rewrite error handling must preserve router-owned meanings such as `MultipleInitializations` and `PortContractMismatch` |
| Typed `PortName` | Compiler catches string errors | Rewrite docs and code must refer to typed port constants, never raw string port names |

Host policy tooling may additionally enforce:

- no edits to frozen files
- `router.lock` integrity before other checks
- `--router-lock-update` as a human-only workflow
- no recommendations that solve host wiring problems by editing frozen router files

### 3.6 Custom Rules Group

The custom group allows users to define regex-based rules in `policy-gate.toml` without writing any Go code. Each rule specifies a pattern, a scope, a severity, and the message to emit on a match.

```toml
[[custom_rules]]
id       = "no-direct-db-call"
message  = "direct database call in service layer; use repository interface"
pattern  = 'db\.Query|db\.Exec'
severity = "error"            # error | warn
file_glob = "internal/app/**/*.go"
language  = "go"              # go | python | typescript | any
enabled   = true
```

**Scoping behaviour:**
- If `file_glob` is provided it is resolved against the repo root
- If `file_glob` is omitted the rule applies to all files under `paths.production_roots`
- If `language` is set only files with the matching extension are checked
- `language = "any"` matches all files regardless of extension

**Implementation:** `custom_rules.go` is a loop over configured rules — compile pattern, walk matching files, regex match per file, emit violation with user-provided message and severity. No AST, no subprocess, no catalog. The entire implementation is under 150 lines.

**Validation:** `config_manager.go` compiles all custom rule patterns at load time and returns an error for any invalid regex, identical to how `secret_logging.allowed_literal_patterns` is handled today.

### 3.7 Config Extensions

The following fields are added to `policy-gate.toml` to close the hardcoded-behaviour gaps. All new fields have defaults matching current behaviour, so existing config files work without modification.

```toml
[go_version]
# Default: ["1.24", "1.25"]
allowed_prefixes = ["1.24", "1.25"]

[hygiene]
# Default: derived from paths.production_roots
scan_roots = ["internal", "cmd"]
# Default: ["cmd/policycheck"]
exclude_prefixes = ["cmd/policycheck"]
# Default: 2
min_name_tokens = 2

[package_rules]
# Default: ["cmd", "internal", "test"]
scan_roots = ["cmd", "internal", "test"]
# Default: 10
max_production_files = 10
# Default: 1
min_concerns = 1
# Default: 2
max_concerns = 2

[function_quality]
# Added to existing section. Default: ["go", "python", "typescript"]
enabled_languages = ["go", "python", "typescript"]
# Recalibrated thresholds (see Section 2.5)
mild_ctx_min               = 12
elevated_ctx_min           = 14
immediate_refactor_ctx_min = 16
error_ctx_min              = 18
error_ctx_and_loc_ctx      = 10

[secret_logging]
# Added to existing section.
# Default: ["example","sample","placeholder","dummy","fake","fixture","redacted","masked"]
benign_hints = ["example", "sample", "placeholder", "dummy", "fake",
                "fixture", "redacted", "masked"]
# Default: ["<token>","<password>","<secret>","<api-key>","changeme","change_me","replace_me"]
placeholder_strings = ["<token>", "<password>", "<secret>", "<api-key>",
                       "changeme", "change_me", "replace_me", "your_token_here"]

[ai_compatibility]
# Default: ["--ai", "--user"]  (both required, matching current behaviour)
required_flags = ["--ai", "--user"]

[scope_guard]
# Default: "ScopeProjectRepo"
required_constant = "ScopeProjectRepo"
# Default: ["os.WriteFile", "os.Rename"]
forbidden_calls = ["os.WriteFile", "os.Rename"]

# Custom rules — zero or more entries
[[custom_rules]]
id        = ""
message   = ""
pattern   = ""
severity  = "warn"
file_glob = ""
language  = "any"
enabled   = true
```

---

## 4. Coding Standards

These standards apply to all code written in the rewrite. They exist to keep the codebase readable, debuggable, and — critically — passing its own policy checks when run against itself.

### 4.1 Nesting Depth

Control flow must not exceed **2 layers of nesting**. A `for` containing an `if` is depth 2 and is the maximum. An `if` inside that `if`, or a `for` inside a `for` inside an `if`, is depth 3 and is banned.

```go
// GOOD — depth 2
for _, entry := range entries {
    if entry.IsDir() {
        continue
    }
    process(entry)
}

// BAD — depth 3, banned
for _, entry := range entries {
    if !entry.IsDir() {
        for _, file := range entry.Files() {   // depth 3
            if file.IsGo() {
                process(file)
            }
        }
    }
}
```

**The escape hatch is a named helper function.** Extracting the inner logic to a well-named function resets the depth counter and costs nothing in terms of cognitive complexity — method calls are free. When you find yourself at depth 3, that is the signal to extract, not to continue nesting.

### 4.2 No Else After Return

If a branch ends in `return`, `continue`, or `break`, the following block must not use `else`. Use the early return / guard clause pattern instead.

```go
// BAD
if err != nil {
    return fmt.Errorf("readFile: %w", err)
} else {
    process(content)   // else after return, banned
}

// GOOD
if err != nil {
    return fmt.Errorf("readFile: %w", err)
}
process(content)
```

### 4.3 Cognitive Complexity Targets

| Target | CTX | Notes |
|---|---|---|
| Ideal | ≤ 8 | Comfortable headroom, easy to test |
| Acceptable | 9 – 11 | Fine, no action needed |
| Mild warning | 12 – 13 | Consider extracting a helper |
| Elevated warning | 14 – 15 | Plan a refactor |
| Immediate refactor | 16 – 17 | Refactor before merge |
| Hard error | ≥ 18 | Blocked, must be reduced |

Functions in the rewrite should target ≤ 8 CTX. This gives headroom against the thresholds the checker itself enforces, so the binary passes its own checks cleanly.

### 4.4 No Silent Errors

Every error must be propagated or logged. Swallowing an error — even temporarily during development — is banned. Silent failures produce wrong output with no indication of why, which is significantly harder to debug than a noisy failure.

```go
// BANNED — silent swallow
if err != nil {
    return nil
}

// BANNED — explicit discard
_ = someFunc()

// BANNED — ignored error return value
result, _ := parseFile(path)

// ACCEPTABLE — bare propagation
if err != nil {
    return err
}

// PREFERRED — wrapped with caller context
if err != nil {
    return fmt.Errorf("checkGoVersion: %w", err)
}
```

The wrapping convention is `"callerFuncName: %w"`. It does not need to be a full descriptive sentence — the function name prefix is enough to trace the call path. Custom error types and full error handling are addressed separately; this rule only requires that errors are never dropped.

### 4.5 No Repeated Nil Guards

Checking the same identifier for `nil` more than once within a single function is a code smell. It signals that neither the caller nor the callee is certain about the value's lifetime, so both check defensively. Resolve the ownership question instead of adding a second check.

```go
// BAD — cfg nil-checked twice in the same function
func process(cfg *Config) error {
    if cfg == nil {
        return nil
    }
    // ... some logic ...
    if cfg == nil {           // repeated nil guard, banned
        return nil
    }
    return cfg.Run()
}

// GOOD — check once at the entry point, trust it thereafter
func process(cfg *Config) error {
    if cfg == nil {
        return fmt.Errorf("process: cfg must not be nil")
    }
    return cfg.Run()
}
```

The checker enforces this at `nil_guard_repeat_warn_count = 8`. The standard here is stricter: more than 2 nil guards on the same identifier within one function is a smell that warrants immediate review regardless of the configured threshold.

### 4.6 Summary

| Rule | DO | DON'T |
|---|---|---|
| Nesting | Extract to named helpers when depth would exceed 2 | Nest control flow beyond 2 layers |
| Branching | Use early returns and guard clauses | Use `else` after `return` / `continue` / `break` |
| Complexity | Target CTX ≤ 8 per function, hard limit at 18 | Write functions that exceed CTX 18 |
| Errors | Propagate with `fmt.Errorf("funcName: %w", err)` | Swallow, discard, or ignore errors silently |
| Nil guards | Check nil once at the entry point | Nil-check the same identifier repeatedly |

---

## 5. Full Config Struct Map

All config structs after the rewrite. Fields marked `// NEW` are additions; all others exist in the current implementation.

```go
type PolicyConfig struct {
    Paths                PolicyPathsConfig                `toml:"paths"`
    FileSize             PolicyFileSizeConfig             `toml:"file_size"`
    FunctionQuality      PolicyFunctionQualityConfig      `toml:"function_quality"`
    Output               PolicyOutputConfig               `toml:"output"`
    SecretLogging        PolicySecretLoggingConfig        `toml:"secret_logging"`
    CLIFormatter         PolicyCLIFormatterConfig         `toml:"cli_formatter"`
    HardcodedRuntimeKnob PolicyHardcodedRuntimeKnobConfig `toml:"hardcoded_runtime_knob"`
    Architecture         PolicyArchitectureConfig         `toml:"architecture"`
    GoVersion            PolicyGoVersionConfig            `toml:"go_version"`        // NEW
    Hygiene              PolicyHygieneConfig              `toml:"hygiene"`           // NEW
    PackageRules         PolicyPackageRulesConfig         `toml:"package_rules"`     // NEW
    AICompatibility      PolicyAICompatibilityConfig      `toml:"ai_compatibility"`  // NEW
    ScopeGuard           PolicyScopeGuardConfig           `toml:"scope_guard"`       // NEW
    CustomRules          []PolicyCustomRule               `toml:"custom_rules"`      // NEW
    Runtime              PolicyConfigMetadata             `toml:"-"`
}

// NEW
type PolicyGoVersionConfig struct {
    AllowedPrefixes []string `toml:"allowed_prefixes"`
}

// NEW
type PolicyHygieneConfig struct {
    ScanRoots       []string `toml:"scan_roots"`
    ExcludePrefixes []string `toml:"exclude_prefixes"`
    MinNameTokens   int      `toml:"min_name_tokens"`
}

// NEW
type PolicyPackageRulesConfig struct {
    ScanRoots          []string `toml:"scan_roots"`
    MaxProductionFiles int      `toml:"max_production_files"`
    MinConcerns        int      `toml:"min_concerns"`
    MaxConcerns        int      `toml:"max_concerns"`
}

// NEW
type PolicyAICompatibilityConfig struct {
    RequiredFlags []string `toml:"required_flags"`
}

// NEW
type PolicyScopeGuardConfig struct {
    RequiredConstant string   `toml:"required_constant"`
    ForbiddenCalls   []string `toml:"forbidden_calls"`
}

// NEW
type PolicyCustomRule struct {
    ID       string `toml:"id"`
    Message  string `toml:"message"`
    Pattern  string `toml:"pattern"`
    Severity string `toml:"severity"`
    FileGlob string `toml:"file_glob"`
    Language string `toml:"language"`
    Enabled  bool   `toml:"enabled"`

    CompiledPattern *regexp.Regexp `toml:"-"` // compiled at config load time
}

// Additions to existing PolicyFunctionQualityConfig — NEW fields only:
//   EnabledLanguages []string `toml:"enabled_languages"`

// Additions to existing PolicySecretLoggingConfig — NEW fields only:
//   BenignHints        []string `toml:"benign_hints"`
//   PlaceholderStrings []string `toml:"placeholder_strings"`
```

`ApplyPolicyConfigDefaults` and `ValidatePolicyConfig` in `config/config_manager.go` are extended to cover each new struct, following the existing `applyDefaultSlice` / `applyDefaultInt` pattern.

---

## 6. Test Strategy

### 6.1 Framework

- `testify/assert` — non-fatal assertions, primary workhorse
- `testify/require` — fatal assertions for setup steps (parse failure, nil result struct)
- `testify/suite` — for Policy 5 and 6, which need shared temp directory construction
- `testify/mock` — only if the scanner dispatcher is extracted behind an interface

### 6.2 Two Tiers

| Tier | Description |
|---|---|
| **Unit — pure functions** | No filesystem. Construct inputs (strings, AST nodes, `[]PolicyFact`, config structs) directly. Use `assert` throughout. Run in milliseconds, exhaustively parameterised with table-driven tests. |
| **Integration — `Check*` functions** | Use `t.TempDir()` to construct a minimal real directory tree. Use `require` for setup, `assert` for violation assertions. |

### 6.3 Test File Layout

```
internal/tests/policycheck/
├── core/
│   ├── contracts/
│   │   ├── go_version_test.go
│   │   ├── cli_formatter_test.go
│   │   ├── ai_compatibility_test.go
│   │   └── scope_guard_test.go
│   ├── quality/
│   │   ├── file_size_test.go       – threshold math (pure) + file walk
│   │   └── func_quality_test.go    – evaluateFunctionQualityFacts (pure)
│   ├── security/
│   │   └── secret_test.go          – pure logic + catalog unit tests
│   ├── hygiene/
│   │   ├── symbol_names_test.go    – tokeniser (pure) + walk
│   │   └── doc_style_test.go       – pure AST checks + walk
│   ├── structure/
│   │   ├── test_location_test.go
│   │   ├── package_rules_test.go   – parseDocGoConcerns (pure) + walk
│   │   └── architecture_test.go
│   ├── custom/
│   │   └── custom_rules_test.go    – pattern matching (pure) + walk
│   └── warnings_test.go            – warning compression logic (pure)
├── config/
│   ├── defaults_test.go
│   ├── validation_test.go
│   └── loader_test.go
└── walk/
    └── walk_test.go
```

### 6.4 Self-Check Acceptance Criterion

The rewrite is complete when the binary can run against its own source root and produce zero violations and zero warnings. This is the single most meaningful acceptance test — it means the code was written to the same standards it enforces.

```bash
go run ./cmd/policycheck --root . --config policy-gate.toml
# expected: policycheck: ok
```

### 6.5 Key Test Cases Per Policy

For each policy, both a passing and a failing fixture must exist for every boundary. Table-driven tests are mandatory for policies with multiple thresholds (5, 6).

---

#### Policy 1 — Go Version

| Input | Expected |
|---|---|
| `"go 1.24.0"` | pass |
| `"go 1.25.3"` | pass |
| `"go 1.23.1"` | violation |
| `"go 1.26.0"` with default config | violation |
| `"go 1.26.0"` with `allowed_prefixes = ["1.26"]` | pass |
| Missing `go` directive | violation |
| Malformed `go.mod` | error returned, no panic |

---

#### Policy 2 — Secret Logging

- Each keyword in the default list triggers a violation when present in a log literal
- Each built-in regex pattern triggers a violation (at least one representative per severity tier: LOW, MEDIUM, HIGH, CRITICAL)
- Variable named `apiKey` passed to log (not a string literal) → pass
- Literal containing a keyword but also containing `"example"` → pass (benign hint)
- `benign_hints` overridden to `[]` in config → benign literals no longer suppressed
- Literal on `allowlist.literal_patterns` → pass
- Pattern ID on `allowlist.pattern_ids` → pass
- Override sets severity `"OFF"` for a pattern ID → finding suppressed
- File under `ignore_path_prefixes` → not scanned
- Same literal outside `secret_scan_roots` → not flagged

---

#### Policy 3 — Test Location

| File path | Expected |
|---|---|
| `internal/tests/foo_test.go` | pass |
| `cmd/policycheck/foo_test.go` | pass (second allowed prefix) |
| `internal/auth/foo_test.go` | violation |
| `cmd/othercmd/foo_test.go` | violation |
| Non-`_test.go` file anywhere | not checked |

---

#### Policy 4 — CLI Formatter

- File in `required_files` with `fmt.Println` → violation
- File in `required_files` with audience-aware formatter only → pass
- `fmt.Sprintf` for string building (not output) in a required file → pass
- File not in `required_files` with `fmt.Println` → pass
- `required_files` empty → no violations

---

#### Policy 5 — File Size (table-driven)

**Threshold formula:**

```
effective_warn = max(warn_loc - (ctx_funcs × warn_penalty), min_warn_loc)
effective_max  = max(max_loc  - (ctx_funcs × max_penalty),  min_max_loc)
constraint:    effective_max >= effective_warn + min_warn_to_max_gap
```

| ctx funcs | effective_warn | effective_max | 650 lines | 701 lines | 826 lines | 901 lines |
|---|---|---|---|---|---|---|
| 0 | 700 | 900 | pass | warn | warn | error |
| 5 | 650 | 825 | pass | warn | error | error |
| 20 (floors) | 450 | 650 | warn | error | error | error |

Additional: gap enforcement boundary, `loc_ignore_prefixes`, files outside `file_loc_roots`.

---

#### Policy 6 — Function Quality (table-driven, recalibrated thresholds)

| ctx | loc | Expected |
|---|---|---|
| 18 | 50 | ERROR (ctx alone) |
| 5 | 120 | ERROR (loc alone) |
| 10 | 80 | ERROR (combined) |
| 10 | 79 | pass (combined: loc just under) |
| 16 | 5 | WARN immediate-refactor |
| 14 | 5 | WARN elevated |
| 12 | 5 | WARN mild |
| 5 | 80 | WARN (LOC in warn band) |
| 11 | 79 | pass |

Nil guard: 7 repeats → pass, 8 repeats → WARN.
Language gating: `enabled_languages = ["go"]` → Python/TS not dispatched to scanner.

---

#### Policy 7 — Symbol Names

| Name | Tokens | Result |
|---|---|---|
| `ValidateSchema` | `[Validate, Schema]` | pass |
| `parseGoAST` | `[parse, Go, AST]` | pass |
| `HTTPHandler` | `[HTTP, Handler]` | pass |
| `validate` | `[validate]` | violation |
| `v` | `[v]` | violation |

Config override: `min_name_tokens = 3` → 2-token names now violate.
Files outside `hygiene.scan_roots` → not checked.

---

#### Policy 8 — Doc Style

| Scenario | Expected |
|---|---|
| `// ValidateSchema validates the schema` on exported func | pass |
| `// This validates the schema` on exported func | violation |
| No comment on exported func | violation |
| Unexported func with no comment | pass |
| `_test.go`, `zz_generated*`, `*.gen.go`, `*_mock.go` | all skipped |

---

#### Policy 9 — AI Compatibility

- File contains both `--ai` and `--user` → pass
- File contains `--ai` but not `--user` (default config) → violation
- `required_flags = ["--ai"]` → file with only `--ai` passes
- Thin wrapper (contains `RunCLI(`) without flags → resolution continues to next candidate

---

#### Policy 10 — Scope Guard

- Target file contains `"ScopeProjectRepo"` → pass
- Target file missing `"ScopeProjectRepo"` → violation
- `required_constant = "ScopeWorkspace"` → checks for that string instead
- `lifecycle_docs` target with `os.WriteFile` → violation
- `forbidden_calls = []` → lifecycle boundary check produces no violations

---

#### Policy 11 — Package Rules

- 9 production `.go` + `doc.go` → pass
- 11 production `.go` + `doc.go` → violation
- `max_production_files = 15` → 11 files passes
- `doc.go` absent → violation
- `doc.go` with 0 bullets → violation
- `doc.go` with 3 bullets and `max_concerns = 2` → violation
- `_test.go` files not counted toward production file limit

---

#### Policy 12 — Architecture Roots

- Child in `allowed_children` → pass
- Child not in `allowed_children` → violation
- Child in `ignore_children` → pass
- `enforce = false` → no violations emitted

---

#### Custom Rules

- Rule with valid pattern matching file content → violation with user message and severity
- Rule with valid pattern, no match → pass
- `enabled = false` → rule not evaluated
- `language = "go"` applied to `.py` file → not evaluated
- `file_glob` scoped to `internal/app/` applied to `internal/db/` file → not evaluated
- Invalid regex pattern → config load error, not a runtime panic
- Empty `custom_rules` list → no violations, no errors

---

## 7. Migration Path

Four ordered phases. Each phase leaves the binary in a working state before the next begins.

### Phase 1 — Config Extension

Add new config structs and fields to `config/config_manager.go`. Extend `ApplyDefaults` and `Validate`. Write config unit tests. No check behaviour changes at the end of this phase — the checker runs identically to before but with a richer config surface.

Deliverables: new structs, extended validation, `config/defaults_test.go` and `config/validation_test.go` passing.

### Phase 2 — Walk Package and Scanner Package

Implement the router-backed host capabilities for walk and scanners, plus the
policy-engine helper wrappers that consume them. Write tests for both. Neither is
called by any existing check yet — they are standalone additions.

Deliverables: walk/scanner adapter tests and any policy-engine host helper tests passing.

### Phase 3 — Group Restructure and Pure Function Extraction

For each policy group in order:

1. Create the group package directory
2. Move and split files (`security.go` → `secret.go` + `testlocation.go` + `clifmt.go`, `quality.go` → `file_size.go` + `func_quality.go`)
3. Extract pure logic functions from each `Check*` orchestrator
4. Rewrite orchestrators to use the walk package
5. Write unit tests for pure functions
6. Write integration tests for orchestrators

After each group, all existing tests must pass before moving to the next.

Deliverables: all six group packages complete, `security.go` and `quality.go` deleted, all tests green.

### Phase 4 — Config Wire-Through and Self-Check

Connect new config fields to the checks that previously hardcoded their values:

- `GoVersion.AllowedPrefixes` → `contracts/go_version.go`
- `Hygiene.*` → `hygiene/symbol_names.go` and `hygiene/doc_style.go`
- `PackageRules.*` → `structure/package_rules.go`
- `FunctionQuality.EnabledLanguages` → `quality/func_quality.go`
- `SecretLogging.BenignHints`, `SecretLogging.PlaceholderStrings` → `security/secret_scan.go`
- `AICompatibility.RequiredFlags` → `contracts/ai_compatibility.go`
- `ScopeGuard.*` → `contracts/scope_guard.go`

For each wire-through, add a config-override test that verifies the behaviour changes when the config value changes. This is the proof that the hardcoding is actually gone.

Final step: run the binary against its own source root. Output must be `policycheck: ok`.

Deliverables: all hardcoded values replaced, config-override tests passing, self-check passing.

### Phase 5 — Router Host Finalisation

Move all remaining host-capability seams onto the router-backed contracts and
verify the final command flow:

- `main.go` boots router before config load
- config source resolution goes through `ConfigProvider`
- directory traversal goes through `WalkProvider`
- external scanner execution goes through `ScannerProvider`
- policy packages remain router-agnostic

Deliverables: router wiring complete, no direct stdlib walk/scanner/config
bootstrapping left in policy-engine orchestration, self-check still passing.

---

## 8. What Does Not Change

- **All policy violation message strings** — exact wording preserved for any downstream tooling that parses output
- **`policy-gate.toml` format** — fully backward compatible; all new fields are optional with defaults
- **`RunPolicyChecks` signature** — same inputs and outputs
- **`types/` package** — `Violation`, `PolicyFact`, `FunctionQualityWarning`, `PolicyCheckResults`, `ScannerBytes`
- **`utils/` package** — `RelOrAbs`, `HasPrefix`, `NormalizePolicyPath`, `CountLines`, `PathExists`
- **`python_scanner.py`** — unchanged
- **`typescript_scanner.ts`** — unchanged
- **CLI flags and output format** — unchanged
