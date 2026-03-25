# Policycheck Target Skeleton

This document satisfies Task T2 from the Rewrite Tasklist Plan A. It defines the target folder structure, ports, and groups before any implementation changes are made.

## Target Package Structure

```
cmd/policycheck/
└── main.go                 // Thin entry point; boots router once, resolves host startup flow

internal/policycheck/
├── cli/
│   ├── doc.go
│   ├── errors.go           // Error formatting and exit code determination
│   ├── warnings.go         // Warning output formatting
│   └── rules.go            // Flag parsing, config loading, result output
├── config/
│   ├── doc.go
│   ├── config_manager.go   // ApplyDefaults, Validate, cross-field checks
│   └── config_loader.go    // Decode/compile raw config source supplied by the config port
├── core/
│   ├── doc.go
│   ├── policy_manager.go   // RunPolicyChecks orchestrator; resolves required ports via small host seam
│   ├── policy_registry.go  // Policy group registration and dispatch
│   ├── contracts/          // Group 1: version, CLI formatter, AI compat, scope guard
│   │   ├── doc.go
│   │   ├── go_version.go
│   │   ├── cli_formatter.go
│   │   ├── ai_compatibility.go
│   │   └── scope_guard.go
│   ├── quality/            // Group 2: file size, function quality
│   │   ├── doc.go
│   │   ├── file_size.go
│   │   └── func_quality.go
│   ├── security/           // Group 3: secret logging
│   │   ├── doc.go
│   │   ├── secret_scan.go
│   │   └── secret_catalog.go
│   ├── hygiene/            // Group 4: symbol names, doc style
│   │   ├── doc.go
│   │   ├── symbol_names.go
│   │   └── doc_style.go
│   ├── structure/          // Group 5: test location, package rules, architecture roots
│   │   ├── doc.go
│   │   ├── test_location.go
│   │   ├── package_rules.go
│   │   └── architecture.go
│   └── custom/             // Group 6: regex-based rules
│       ├── doc.go
│       ├── custom_rules.go
│       └── rule_matcher.go
└── host/
    ├── ports.go            // Typed host seam
    └── bootstrap.go        // Host boot helper
```

## Router-Facing Capabilities (Ports)

These interfaces define the host capabilities required by the policy checks. They will be resolved via the router.

```
internal/ports/
├── config.go   // PortConfig
├── walk.go     // PortWalk
└── scanners.go // PortScanner
```

## Concrete Adapters

These implement the ports and are wired into the router. Adapters must never import each other.

```
internal/adapters/
├── config/
├── walk/
└── scanners/
```

## Target Test Structure

Tests will mirror the pure logic and orchestration boundaries.

```
internal/tests/policycheck/
├── config/
├── core/
│   ├── contracts/
│   ├── quality/
│   ├── security/
│   ├── hygiene/
│   ├── structure/
│   └── custom/
├── host/
└── walk/
```

## Core Hexagonal Principles

- **Direction of Dependencies:**
  - `consumer` -> `internal/ports` + `internal/router`
  - `host boot` -> `internal/router/ext`
  - `router wiring` -> `internal/adapters/*`
- **Adapters:** Independent of one another. Adapters importing adapters is strictly prohibited.
- **Router Use:** Router is complete infrastructure. Core components do not bypass port boundaries. Manual edits to `internal/router` files are prohibited; all new ports will be added using `go run ./internal/router/tools/wrlk add ...`.
