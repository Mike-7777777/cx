# Contributing to cx

Thanks for your interest in contributing!

## Development Setup

```bash
git clone https://github.com/Mike-7777777/cx.git
cd cx
make build    # build binary
make test     # run tests
make lint     # run golangci-lint
```

Requires Go 1.24+ and [golangci-lint](https://golangci-lint.run/welcome/install/).

## Workflow: Before Every Commit

```bash
# 1. Format
gofmt -w .

# 2. Build
go build -o cx.exe .          # Windows
go build -o cx .              # Unix

# 3. Test
go test ./... -count=1

# 4. Lint (must match CI)
golangci-lint run

# 5. Only commit if all 4 pass
```

Or use the Makefile shortcuts:

```bash
make lint test build    # all three in one
```

## Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

| Prefix | When to use | Example |
|--------|-------------|---------|
| `feat:` | New command or feature | `feat: cx sessions — list sessions across accounts` |
| `fix:` | Bug fix | `fix: statusline path broken on Windows` |
| `refactor:` | Code change that doesn't fix a bug or add a feature | `refactor: centralize UI labels` |
| `docs:` | Documentation only | `docs: update README with new commands` |
| `chore:` | Build, CI, tooling | `chore: relax golangci-lint rules` |
| `revert:` | Undo a previous change | `revert: remove i18n locale feature` |

GoReleaser uses these prefixes to auto-generate the changelog on release.

## Adding a New Command

1. Create `run_<name>.go` with a `xxxCmd` struct implementing `Runner`
2. Register in `main.go`:
   - Add `&xxxCmd{}` to `commands` map with category
   - Add usage hint to `commandUsageHint` if it takes args
3. Add tests in `run_<name>_test.go`
4. Add to README.md command table
5. Add to CHANGELOG.md under `[Unreleased]`
6. Run full check: `gofmt -w . && make lint test build`

Example skeleton:

```go
// run_example.go
package main

import (
    "context"
    "fmt"
)

type exampleCmd struct{}

func (c *exampleCmd) Run(_ context.Context, app *App, args []string) error {
    fmt.Fprintln(app.Stdout, "hello")
    return nil
}
```

```go
// main.go — add to commands map:
"example": {&exampleCmd{}, "Short description", catDailyUse},
```

## Adding a New UI Label

All user-facing strings ("5h", "7d", "ctx", "reset", etc.) live in `internal/format/labels.go`. Add new labels there, reference them as `format.LabelXxx` everywhere.

## Code Rules

- **No hardcoded paths** — use `os.UserHomeDir()`, `filepath.Join()`, `$CLAUDE_PROJECT_DIR`
- **No personal data** — no emails, names, account IDs in source code
- **No hardcoded UI strings** — use `format.Label*` constants
- **Write to `app.Stdout`/`app.Stderr`** — never `os.Stdout`/`os.Stderr` in commands
- **Return `error` from `Run`** — never call `os.Exit` in commands
- **Forward slashes** in any path that goes into settings.json or shell commands
- **`internal/`** for all packages — nothing exported outside the binary
- **Minimal dependencies** — only `natefinch/atomic` as external dep
- **`any` not `interface{}`** — use modern Go syntax

## CI Requirements

CI runs on push to `main` (3 jobs in parallel):
- **lint**: `golangci-lint run` on Ubuntu
- **test**: `go test -v -count=1 -race` on Ubuntu / macOS / Windows
- **build**: `go build` + `go vet` on all 3 platforms

All 3 must pass. If lint fails, check `.golangci.yml` for the rule and either fix the code or add an exclusion with a comment explaining why.

## Binary Update on Windows

If cx.exe is locked (CC statusline is using it), swap instead of overwrite:

```bash
go build -o cx-new.exe .
mv cx.exe cx-old.exe
mv cx-new.exe cx.exe
rm cx-old.exe
```

## Reporting Issues

Open a [GitHub Issue](https://github.com/Mike-7777777/cx/issues) with:
- cx version (`cx version`)
- OS and architecture
- Steps to reproduce
- Expected vs actual behavior
