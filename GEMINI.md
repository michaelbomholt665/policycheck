# AGENTS.md

Guidance for AI agents working in this repository.

---

1. **Run the policy checker**: `go run ./cmd/policycheck --policy-list` to see all enforced rules.

---

## Formatting

- **Go**: Always `gofumpt -l -w .` — **not** `gofmt`. Run after every edit batch.
- **Python**: `ruff check scripts/` + `ruff format scripts/`.
- **TypeScript**: `tsc --noEmit` for checks, plus `npm run build:scanner` to regenerate `dist/scanner.cjs`.

---

## DO

- **Do** use `flag.NewFlagSet` for every subcommand — no global `flag.Parse()`.
- **Do** add `--config` to every new `Run*` function — default `"policy-gate.toml"`, always override-able.
- **Do** wrap all errors: `fmt.Errorf("context: %w", err)` — bare `return err` is a bug.
- **Do** defer `db.Close()`, `rows.Close()`, `stmt.Close()` — check the existing pattern in `backup.go`.
- **Do** put package-level constants in `UPPER_CASE` and compile-once `regexp` at package level.
- **Do** write doc comments on every exported symbol: Google-style, full sentence, capital letter.
- **Do** write package `doc.go` for every package with `// Package X` starting the block comment.
- **Do** use `log.Printf` (not `fmt.Println`) for diagnostic/error messages inside goroutines.
- **Do** keep function cognitive complexity ≤ 15 — the policy checker enforces this.
- **Do** name functions with ≥ 2 tokens: `RunBackup`, not `backup`.
- **Do** use `observationVersion` constant (from `internal/adapters/types.go`) — not a raw `"0.1.0"` string.
- **Do** run `go run ./cmd/policycheck` 1–3 times during any plan implementation.

## DON'T

- **Don't** use `gofmt` — use `gofumpt`.
- **Don't** use CLI frameworks (Cobra, Kong, urfave/cli). `flag` package only.
- **Don't** add global state — config is loaded per-command and passed by value.
- **Don't** hardcode the config path — always via `fs.String("config", "policy-gate.toml", ...)`.
- **Don't** write `regexp.MustCompile(...)` inside a function body — package-level only.
- **Don't** call `policy-gate.toml` as a source of truth for anything beyond the policy/config data the app actually reads.
- **Don't** create a `schema_id` at runtime without a matching TOML declaration.
- **Don't** write tests inside `internal/adapters/` — all tests go in `internal/tests/`.
- **Don't** write directly to `os.Stdout` in adapters — use `fmt.Fprintf(os.Stdout, ...)` so it can be redirected in tests.
- **Don't** use `bash` as the executor for scripts — the dispatcher calls `python`/`python3` and `node` directly.
- **Don't** import third-party packages in `scripts/scanner.py` — stdlib `ast` only.
- **Don't** touch `docs/Initial/` — those are locked design records, not living documents.

---

## Policy Checks

```powershell
go run ./cmd/policycheck               # check current state
go run ./cmd/policycheck --policy-list # list all enforced rules
```

Run 1–3 times per implementation session. Always run before declaring completion.

---

## Build & Test

```powershell
make build   # npm run build:scanner + go build ./...
make lint    # golangci-lint run + ruff check scripts/ + tsc --noEmit
make clean   # remove binary artifacts

go test ./internal/tests/... -v -count=1   # all Go tests (in-memory SQLite)
python scripts/scanner_test.py -v          # Python scanner tests
```

---

## Toolchain Requirements

| Tool            | Minimum | Notes                                                                   |
| --------------- | ------- | ----------------------------------------------------------------------- |
| Go              | 1.24    | `go/ast`, range-over-func                                               |
| Python          | 3.12    | `ast` stdlib; `uv` runner preferred, `python3` fallback                 |
| Node            | 20 LTS  | Runs embedded `dist/scanner.cjs`; TypeScript is a build-time dependency |
| `golangci-lint` | latest  | Enforced in `make lint`                                                 |
| `gofumpt`       | latest  | Required formatter — not gofmt                                          |
| `ruff`          | latest  | Python linter + formatter                                               |

---

## Key Conventions

### CLI
- No frameworks. Every subcommand is a `func(args []string) error` registered in `internal/app/run.go`.
- Subcommand flags are parsed with `flag.NewFlagSet(name, flag.ContinueOnError)` inside the function.

### Config
- `policy-gate.toml` is loaded fresh on each command — no caching, no singleton.
- `[registry.contracts]` is the only authoritative source for contract identities.
- `[registry.domains]` must be non-empty if any contracts are declared — config loader rejects at startup.

### NDJSON Observation Format
Every scanner outputs one JSON object per line:
```json
{
  "kind": "symbol_observation",
  "version": "0.1.0",
  "symbol_name": "list_notes",
  "qualified_name": "src.routes.notes.list_notes",
  "language": "python",
  "symbol_kind": "function",
  "file_path": "src/routes/notes.py",
  "line_number": 42,
  "end_line": 58,
  "signature_text": "def list_notes(project_id: int) -> list[Note]:",
  "observation_hash": "<sha256(file_path:symbol_name:line_number)>"
}
```

### target_ref Format (enforced by `config.ValidateTargetRef`)
| target_type | format                                | example                                    |
| ----------- | ------------------------------------- | ------------------------------------------ |
| `symbol`    | `<lang>:<file_path>:<qualified_name>` | `go:internal/db/queries.go:db.GetUserByID` |
| `file`      | `file:<path>`                         | `file:src/routes/notes.py`                 |
| `table`     | `table:<name>`                        | `table:public.users`                       |
| `route`     | `route:<method>:<path>`               | `route:GET:/api/v1/notes`                  |

### Database
- SQLite only through Phase 2. Driver: `modernc.org/sqlite` (pure Go, no CGO).
- Tests use: `file::memory:?cache=shared`
- Schema applied by `internal/db/schema.go:EnsureSchema` — idempotent (`CREATE TABLE IF NOT EXISTS`).
- No third-party migration library.

### Testing
- All Go tests in `internal/tests/`, mirroring the `internal/` structure.
- Use `github.com/stretchr/testify/assert` and `require`.
- Table-driven tests preferred; subtests for grouped cases.

### Scanners
- **Go**: built into binary via `go/ast` + `go/parser`; no subprocess.
- **Python**: `scripts/scanner.py` — stdlib `ast` only.
- **TypeScript**: `scripts/scanner.ts` builds `dist/scanner.cjs`; policycheck runs the embedded CJS with `node`.
- All scanners write NDJSON to **stdout only** — the Go binary reads and stages it.

---

## File Location Reference

| What                | Where                                                                  |
| ------------------- | ---------------------------------------------------------------------- |
| CLI entry           | `cmd/policycheck/main.go`                                              |
| Dispatch table      | `internal/app/run.go`                                                  |
| Config structs      | `internal/config/config.go`                                            |
| Policy design docs  | `docs/policycheck/`                                                    |
| Router design docs  | `docs/router/`                                                         |
| All tests           | `internal/tests/`                                                      |
| Staged observations | `.policycheck/staging/staged.<lang>.ndjson`                            |
| Exports             | `.policycheck/exports/<filter>_<timestamp>.tsv`                        |
| Embedded scripts    | `.policycheck/scripts/`                                                |
| Design docs         | `docs/Design/01–05_*.md`                                               |
| Codebase report     | `docs/reports/codebase_report.md`                                      |
