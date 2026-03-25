# policycheck - Repository Policy Validator

---

## What This Tool Does

`policycheck` is a local developer policy validator for the repository. It enforces a set of configurable rules (via `policy-gate.toml`) such as:

| Policy Category      | Description                                                                                |
| -------------------- | ------------------------------------------------------------------------------------------ |
| **Go Version**       | Enforces Go version constraints (1.24.*, 1.25.*)                                           |
| **Secret Detection** | Detects string literals containing secret-like keywords or token patterns in logging calls |
| **Test Placement**   | Enforces `*_test.go` files in `internal/tests/` by default                                 |
| **CLI Output**       | Detects raw `fmt.Print*` usage in configured command files                                 |
| **Code Quality**     | File and function size/complexity thresholds (LOC and CTX penalties)                       |
| **Runtime Knobs**    | Hardcoded runtime knob detections (heuristic warnings)                                     |
| **Architecture**     | Architecture root/concern checks                                                           |

---

## Secret Detection Improvements

- The tool now includes **built-in compiled regular expressions** for common token formats:
  - GitHub/GitLab PATs
  - AWS-like keys
  - Google API keys
  - OpenAI/other provider key prefixes
  - Slack tokens
  - PEM private key headers
  - DB connection URIs
  - And more

- These built-in patterns **augment** the existing keyword checks and are applied to string literal arguments of logging calls.

- To avoid false positives, the checker **only inspects literal string arguments** (including those in string concatenations) and ignores variables or constants defined elsewhere. This is a known limitation of simple AST-based scanning without full data-flow analysis.

- `policy-gate.toml` also supports:
  - `secret_logging.ignore_path_prefixes`
  - `secret_logging.allowed_literal_patterns`

> **Tip:** Use path prefixes and config to exclude doc/test fixture directories where token-like example strings may appear.

---

## Quick Start

### Run the Policy Checker

```bash
go run ./cmd/policycheck
```

### List All Enforced Policies

Handy for automation/AI agents:

```bash
go run ./cmd/policycheck --policy-list
```

### Print Architecture Concern Locations

```bash
go run ./cmd/policycheck --concern <name>
```

---

## Configuration

- The tool reads `policy-gate.toml` in the repository root. If the file is missing, a default template (`policy_gate_default.toml`) will be written to the repository root and the tool will exit with an informational message to review and re-run.
- Use `--no-create` or `--dry-run` if you want missing config to fail without writing a file.

- Path- and threshold-based behavior is configurable under the `[paths]`, `[file_size]`, `[function_quality]`, and other sections.

- Default configuration is embedded; you can create or edit `policy-gate.toml` to tune policies for your repo.

### Recommended Configurations

Useful `policy-gate.toml` settings:

| Setting                  | Purpose                              |
| ------------------------ | ------------------------------------ |
| `secret_logging.keywords`                 | Repo-specific secret substrings              |
| `secret_logging.allowed_literal_patterns` | Whitelist known-safe log literals            |
| `secret_logging.ignore_path_prefixes`     | Exclude doc/test fixture directories         |
| `[[architecture.roots]]`                  | Restrict direct children under key roots     |
| `[[architecture.concerns]]`               | Define AI-friendly concern location mappings |

---

## Important Flags

| Flag            | Default            | Description                                               |
| --------------- | ------------------ | --------------------------------------------------------- |
| `--root`        | `.`                | Repository root to scan                                   |
| `--config`      | `policy-gate.toml` | Path to the policy config TOML                            |
| `--policy-list` | —                  | Print list of enforced policies and exit                  |
| `--concern`     | —                  | Print locations for a named architecture concern and exit |
| `--no-create`   | —                  | Fail if config is missing instead of creating it          |
| `--dry-run`     | —                  | Same as `--no-create` for config creation behavior        |

---

## Notes for Automation and AI Workflows

- The tool may create `policy-gate.toml` when missing unless `--no-create` or `--dry-run` is used.

- For AI-guided workflows, prefer to run `--policy-list` first so the AI understands constraints before making edits.

- Consider parsing output (structured output may be added in future) and treating **WARN** entries as guidance while **ERROR** entries represent stronger constraints.

---

## TSV/CSV Exports & Symbol-table Extraction (Preferred for AI)

JSON manifests are useful for ingestion pipelines and human-readable snapshots, but for targeted AI consumption or lightweight tools a compact tabular export (TSV/CSV) is often preferable:

- TSV is compact, easier to slice and filter, and avoids JSON verbosity.
- policycheck (or your manifest tool) can produce exports in `.isr/exports/<filter>_<timestamp>.tsv` containing one row per symbol/observation with a concise set of columns.

Recommended TSV header (example):

snapshot_id	shard_uuid	record_uuid	qualified_name	file_path	line_number	end_line	signature_text	observation_hash	semantic_confirmed	semantic_reference

Example (TSV row):

42	example-shard-00000042	example-rec-0000a1b2	api.users.GetUser	api/users.go	18	26	"func GetUser(ctx context.Context, userID string) (*User, error)"	af0e2c2c8b7a9f4d...	false	

Why use TSV for AI tasks:

- Small, focused slices: you can export only changed symbols or a per-feature subset.
- Easier to load into pandas/numpy for quick analytics without parsing large JSON trees.
- Works well with your parquet + shard model: export a TSV view for a given shard window.

If you later need streaming ingestion, maintain NDJSON or Parquet alongside TSV exports — they serve different use cases.

---

## Recommended Local Workflow for AI-Assisted Changes

1. **Gather rules** — Run `go run ./cmd/policycheck --policy-list`
2. **Generate patch** — AI generates patch according to rules
3. **Validate changes** — Run `go run ./cmd/policycheck`
4. **Iterate** — If violations appear, fix changes and re-run

---

## Guidance to Reduce False Positives

- Add **ignored prefixes (paths)** in `policy-gate.toml` for docs/fixtures where example tokens appear.

- If you have many legitimate literal tokens (e.g., in test fixtures), add `secret_logging.allowed_literal_patterns`.

- Built-in secret patterns are a strong default; prefer to **tune** rather than disable them globally.

---

## Development Notes (for Maintainers)

Consider adding the following improvements to make the tool more automation/AI-friendly:

| Improvement                          | Description                                                                       |
| ------------------------------------ | --------------------------------------------------------------------------------- |
| `--format=json` or `--format=ndjson` | Machine-readable output                                                           |
| Working directory                    | Run external scanners with working dir = repo root                                |
| Node modules                         | Prefer repo-root node_modules discovery                                           |
| AST-based scans                      | Convert brittle string checks to AST-based scans (CLI flags and fmt usage checks) |
| Receiver-aware logging sinks         | Reduce false positives from method-name-only logging detection                    |
| Extra deny patterns                  | Expose additional custom secret regexes in configuration                          |

---

## Extending / Improving

See [`REVIEW.md`](./REVIEW.md) for a detailed review, suggested fixes for false positives, and recommended feature additions:

- `--format=json` for machine-readable results
- AST-based detection for CLI fmt usage and improved logging sink identification
- additional custom secret pattern configuration
- confidence / severity levels for heuristic findings

---

## Optional: NDJSON manifest integration (expansion)

policycheck can be extended to emit or consume NDJSON manifest lines compatible with the repository's manifest builder. This is an optional expansion useful when integrating with larger ingestion/graph pipelines (Parquet shards, Ladybug graph).

- Use the human-readable manifest as the authoritative source for example content: `docs/documentation/manifests/examples/master_snapshot_42.readable.json`.

- A compact NDJSON line (one JSON object per line) example derived from the `GetUser` symbol in that readable JSON (example only):

{"kind":"symbol_observation","snapshot_id":42,"shard_uuid":"example-shard-00000042","record_uuid":"example-rec-0000a1b2","symbol_name":"GetUser","qualified_name":"api.users.GetUser","language":"go","file_path":"api/users.go","line_number":18,"end_line":26,"signature_text":"func GetUser(ctx context.Context, userID string) (*User, error)","observation_hash":"af0e2c2c8b7a9f4d38d1f3e1362188ab52c4c6ec0d6d95ed2c70d89d9b2b1f01","semantic_confirmed":false,"semantic_reference":null}

Notes about the example:

- `shard_uuid` / `record_uuid` illustrate how to reference the parquet shard and the specific record (UUIDv7 or other monotonic ID scheme recommended).
- `semantic_confirmed` indicates whether the observation was resolved against the semantic graph (Ladybug). If `true`, `semantic_reference` should contain a graph node id (e.g. `ladybug:node:123456789`).
- Keep NDJSON lines compact (one object per line) so they are streamable and easily consumable by ingestion tools.

When to use NDJSON integration:

- CI or server-side runners that want to ingest findings into the central graph/metadata pipeline.
- When you want policycheck findings to be joinable with Parquet shards and the semantic graph (auditability and downstream analysis).

This is an optional expansion; policycheck remains useful without it. If you later enable NDJSON emission, keep it opt-in and documented so local developers don't get unexpected file outputs.

---

## Contact / Contribution

This folder is part of the **isr** repository. Follow repository contribution guidelines and formatting rules (`gofumpt`) when editing these files.
