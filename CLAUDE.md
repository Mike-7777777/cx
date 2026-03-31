# CLAUDE.md

Guide for AI contributors working on the cx codebase.

## Project

cx is a Claude Code multi-account manager. It ships as a single Go binary with zero runtime dependencies. Users register multiple Claude Code accounts, switch between them, monitor rate limits, and launch sessions from the best-available account.

Repository: `github.com/Mike-7777777/cx` | Go 1.24+ (CI uses latest stable) | Single external dep: `natefinch/atomic`

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
make check       # fast local gate: fmt + tidy + vet + test + build (~15s)
make ci          # full CI parity: adds golangci-lint
make coverage    # test coverage report
```

Individual targets: `make fmt`, `make tidy`, `make vet`, `make test`, `make lint`, `make build`.

## Git Hooks

```bash
make install-hooks    # one-time: activates pre-commit (gofmt) + pre-push (full check)
```

Hooks live in `scripts/hooks/` (tracked in git). `make install-hooks` sets `core.hooksPath`.

## CI

CI runs on push to `main` and PRs — 5 jobs with concurrency control:

| Job | What |
|-----|------|
| check | gofmt + go mod tidy drift |
| lint | golangci-lint |
| vuln | govulncheck |
| test | 3-platform test + vet (-race) |
| build | 3-platform build + binary size (runs after check+lint+test) |
