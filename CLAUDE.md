# CLAUDE.md

Guide for AI contributors working on the cx codebase.

## Project

cx is a Claude Code multi-account manager. It ships as a single Go binary with zero runtime dependencies. Users register multiple Claude Code accounts, switch between them, monitor rate limits, and launch sessions from the best-available account.

Repository: `github.com/Mike-7777777/cx` | Go 1.24+ | Single external dep: `natefinch/atomic`

## Architecture

Flat main package with one `run_*.go` file per CLI command. Each file exports a single `func run<Name>()` entry point, registered in the `commands` map in `main.go`.

```
main.go              # CLI dispatch, help, version
run_<cmd>.go         # one per command (run, switch, status, setup, ...)
credentials.go       # credential helpers
internal/
  config/            # Registry (account storage), DetectConfigDir
  format/            # Labels, colors, ProgressBar, FormatNumber
  errors/            # Sentinel errors (ErrAccountNotFound, ...)
  cache/             # Rate-limit cache
  usage/             # Usage parsing, aggregation, pricing
  platform/          # OS-specific code (symlinks, exec, ANSI)
  statusline/        # CC status bar parsing and rendering
web/                 # Embedded HTML/JS for the web dashboard
testdata/            # Fixtures for tests
skill/               # Claude Code skill files
```

## Key Patterns

- **`format.Label*`** — all user-facing UI strings live in `internal/format/labels.go`. Never use raw string literals for labels; add a `LabelXxx` constant and reference it.
- **`format.Colorize`** — wrap ANSI color codes; always pass the `enabled` flag.
- **`config.Registry`** — the account registry (`~/.cx.json`). Use `LoadOrCreateRegistry` / `Save`.
- **`config.DetectConfigDir`** — resolves the active Claude Code config directory (env override, XDG, fallback).
- **`internal/errors`** — sentinel errors. Use `errors.Is()` for matching.
- **Adding a command**: create `run_<name>.go`, register in `commands` map + `commandUsageHint` in `main.go`.

## Rules

- Run `gofmt -w .` before every commit.
- No hardcoded paths — use `os.UserHomeDir()`, `filepath.Join()`, env vars.
- No hardcoded UI strings — use `format.Label*` constants.
- Forward slashes in any path written to JSON or shell commands.
- All packages under `internal/` — nothing exported outside the binary.
- Use `any` not `interface{}`.
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
