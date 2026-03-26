# CLAUDE.md

Guide for AI contributors working on the cx codebase.

## Project

cx is a Claude Code multi-account manager. It ships as a single Go binary with zero runtime dependencies. Users register multiple Claude Code accounts, switch between them, monitor rate limits, and launch sessions from the best-available account.

Repository: `github.com/Mike-7777777/cx` | Go 1.24+ | Single external dep: `natefinch/atomic`

## Architecture

Every CLI command implements the `Runner` interface:

```go
type Runner interface {
    Run(ctx context.Context, app *App, args []string) error
}
```

`App` is a dependency injection container holding `Registry`, `Stdout`, `Stderr`, and `UseColor`. Commands receive `App` from `main()` — they never read `os.Args`, call `os.Exit`, or write to `os.Stdout` directly.

```
main.go              # CLI dispatch via Runner interface, buildApp()
app.go               # Runner interface, App struct, parseFlags helper
run_<cmd>.go         # one xxxCmd struct per command
credentials.go       # credential checking helpers
internal/
  config/            # Registry (account storage), DetectConfigDir
  format/            # Labels, colors, ProgressBar, FormatNumber
  errors/            # Sentinel errors (ErrAccountNotFound, ...)
  cache/             # Rate-limit cache (WriteRateCache, ReadRateCache)
  usage/             # JSONL parsing, aggregation, pricing, incremental cache
  platform/          # OS-specific code (symlinks, exec, ANSI)
  statusline/        # CC status bar parsing and rendering
web/                 # Embedded HTML/JS for the web dashboard
skill/               # Embedded Claude Code skill file
testdata/            # Fixtures for tests
```

## Key Patterns

- **Runner pattern** — every command is a struct implementing `Runner`. Tests construct the struct directly with a buffer `App`, no I/O mocking needed.
- **`parseFlags(args, known...)`** — extracts `--key=value` and `--bool` flags from args, returns `(flags map, positional []string)`. Use this instead of reading `os.Args`.
- **`format.Label*`** — all user-facing UI strings live in `internal/format/labels.go`. Never use raw string literals for labels.
- **`format.Colorize`** — wrap ANSI color codes; always pass the `app.UseColor` flag.
- **`config.Registry`** — the account registry (`~/.cx.json`). Use `app.Registry` for reads, `LoadOrCreateRegistry` for writes.
- **`internal/errors`** — sentinel errors. Use `errors.Is()` for matching.
- **Adding a command**: create `run_<name>.go` with `xxxCmd` struct, register `&xxxCmd{}` in `commands` map in `main.go`.

## Rules

- Run `gofmt -w .` before every commit.
- No hardcoded paths — use `os.UserHomeDir()`, `filepath.Join()`, env vars.
- No hardcoded UI strings — use `format.Label*` constants.
- Forward slashes in any path written to JSON or shell commands.
- All packages under `internal/` — nothing exported outside the binary.
- Use `any` not `interface{}`.
- Write output to `app.Stdout`/`app.Stderr`, never `os.Stdout`/`os.Stderr`.
- Return `error` from `Run`, never call `os.Exit`.
- Minimal dependencies — do not add external deps without strong justification.
- Conventional Commits for commit messages (`feat:`, `fix:`, `refactor:`, `docs:`, `chore:`).

## Build / Test / Lint

```bash
go build -o cx.exe .           # Windows
go build -o cx .               # Unix
go test ./... -count=1         # tests (add -race on Unix)
golangci-lint run              # lint (must match CI)
gofmt -w .                     # format
```

Or use the Makefile: `make build test lint`

## CI

CI runs on push to `main` — 3 parallel jobs: lint (Ubuntu), test (Ubuntu/macOS/Windows with -race), build (all 3 platforms). All must pass.
