# policycheck

***UNDER ACTIVE DEVELOPMENT*** 
- This README is NOT complete nor current

A local developer policy validator for Go repositories. policycheck enforces configurable code quality, security, and architecture rules through static analysis.

## What It Does

policycheck scans your repository and enforces policies around:
- Code quality (file size, function complexity)
- Security (secret detection in logs)
- Architecture (package structure, root/concern separation)
- Coding standards (naming, documentation)
- Test placement

## Supported Policy Categories

### 1. Go Version
Ensures the project uses an approved Go version (1.24.x or 1.25.x as configured).

### 2. Secret Detection
Detects string literals containing secret-like keywords or token patterns in logging calls:
- Keywords: `dsn`, `token`, `apikey`, `passwd`, `api_key`, `password`, `connection string`, `connection_string`
- Built-in regex patterns: GitHub/GitLab PATs, AWS keys, Google API keys, OpenAI keys, Slack tokens, PEM private key headers, DB connection URIs
- Only inspects literal string arguments (variables/constants excluded)

Configuration options:
- `secret_logging.keywords` - repo-specific secret substrings
- `secret_logging.ignore_path_prefixes` - exclude paths (e.g., docs/fixtures)
- `secret_logging.allowed_literal_patterns` - whitelist known-safe literals

### 3. Test Location
Enforces `*_test.go` files are in configured directories (default: `internal/tests/`).

### 4. CLI Formatter
Detects raw `fmt.Print*` usage in command files and requires audience-aware formatter usage.

### 5. File Size
Enforces maximum lines per `.go` file with dynamic thresholds based on cognitive complexity:
- Base threshold (configurable)
- Penalty deductions per high-complexity function
- Minimum floor values to prevent over-penalization

### 6. Function Quality
Analyzes Go, Python, and TypeScript functions for:
- **LOC** (lines of code)
- **CTX** (cognitive complexity)

Severity bands:
- **Mild**: Slightly elevated complexity - may be compressed in summaries
- **Elevated**: Listed explicitly, consider refactoring
- **Immediate**: Requires refactoring before merge

### 7. Symbol Names
Function names must have at least 2 tokens (e.g., `ValidateSchema`, not `Check`).

### 8. Documentation Style
Exported functions and types must have Google-style comments starting with the symbol name.

### 9. AI Compatibility
Root command must support `--ai` flag.

### 10. Scope Guard
Commands must default to `ScopeProjectRepo` for the `--scope` flag.

### 11. Package Rules
- Max 10 production files per package
- Each package must have a `doc.go` with a `Package Concerns:` section

### 12. Architecture Roots
Enforces directory structure under configured roots with allowed children.

---

## Quick Start

```bash
# Run the policy checker
go run ./cmd/policycheck

# List the active policy groups
go run ./cmd/policycheck --policy-list

# List the enforced rule catalog
go run ./cmd/policycheck --list-rules

# Browse the policy catalog interactively when supported
go run ./cmd/policycheck --interactive

# Print architecture concern locations
go run ./cmd/policycheck --concern <name>
```

---

## Configuration

policycheck reads `policy-gate.toml` from the repository root. If missing, a default template is created.

### Important Flags

| Flag            | Default            | Description                                      |
| --------------- | ------------------ | ------------------------------------------------ |
| `--root`        | `.`                | Repository root to scan                          |
| `--config`      | `policy-gate.toml` | Path to policy config TOML                       |
| `--policy-list` | —                  | Print list of active policy groups               |
| `--list-rules`  | —                  | Print the enforced rule catalog with descriptions |
| `--interactive` | —                  | Browse policy groups and rules through the router-native interaction capability |
| `--concern`     | —                  | Print locations for a named architecture concern |
| `--no-create`   | —                  | Fail if config is missing                        |
| `--dry-run`     | —                  | Same as `--no-create`                            |
| `--format`      | `text`             | Output format: `text`, `json`, `ndjson`          |

## Router-Native CLI Capabilities

policycheck now uses the split router-native CLI capability surface directly:

- `ResolveCLIOutputStyler()` for structured table output
- `ResolveCLIChromeStyler()` for semantic headings, panels, and violation gutters
- `ResolveCLIInteractor()` for the optional interactive catalog flow

If a CLI capability is unavailable, policycheck degrades to plain non-interactive output.

---

## Development

### Requirements

- Go 1.24+
- Node 20 LTS+ (for TypeScript scanner)
- Python 3.12+ (for Python scanner)

### Build

```powershell
make build
```

### Lint

```powershell
make lint
```

### Test

```powershell
go test ./internal/tests/... -v -count=1
```

---

## Project Structure

```
policycheck/
├── cmd/policycheck/         # CLI entry point and embedded scanners
├── internal/adapters/       # Config adapter wiring
├── internal/ports/          # Port definitions and shared interfaces
├── internal/router/         # Router boot and registry logic
└── internal/tests/router/   # Router-focused test suite
```

---

## Configuration File

The default `policy-gate.toml` includes these sections:

- `[paths]` - directories to scan for each policy type
- `[file_size]` - LOC thresholds and penalties
- `[function_quality]` - complexity thresholds
- `[secret_logging]` - keywords and patterns
- `[cli_formatter]` - required formatter files
- `[hardcoded_runtime_knob]` - runtime knob identifiers
- `[architecture]` - roots and concerns
