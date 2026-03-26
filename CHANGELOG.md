# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- **Runner architecture** — all 17 commands implement `Runner` interface with `Run(ctx, app, args) error`, enabling dependency injection and unit testing
- **App DI container** — `App` struct holds `Registry`, `Stdout`, `Stderr`, `UseColor`; commands never access `os.Args` or `os.Stdout` directly
- **context.Context** — threaded through all commands for graceful shutdown
- **Cross-account session resume** — `cx resume --on <acct>` runs any session on any account; shared `projects/` directory via junction/symlink
- **Session topic display** — `cx sessions` and `cx resume` show first/last user message as topic
- **Claude Code skill** — embedded `skill/cx.md` installed by `cx setup`; use `/cx` inside CC
- **PATH installation** — `cx setup` creates `~/bin/cx` wrapper for non-interactive shells (CC's bash)
- **Output style display** — statusline shows CC mode (e.g., "Fast") when not default
- **Global web tooltips** — fixed-position tooltips in web dashboard, no overflow clipping
- **125+ tests** — switch safety (eval injection), smartScore routing, status output, parseFlags, findWrapperEnd
- `cx sessions` / `cx resume` — list recent CC sessions across all accounts with topic preview
- `cx config` — show/main/rename/set email|alias
- Auto-read identity from `.claude.json` (email, displayName, tier, CC version, session count)
- `cx status` shows subscription tier column, main marker, and 7d reset date
- 7d daily budget indicator in statusline: `~14%/d` (green/yellow/red)
- `cx doctor` auto-fixes stale statusline paths, project-level overrides, and missing junctions
- Graceful statusline fallback when CC doesn't pipe stdin data
- GoReleaser config, golangci-lint, CONTRIBUTING.md, SECURITY.md, Makefile

### Fixed
- `<synthetic>` model entries (CC API errors) no longer pollute model breakdown
- Rate cache write failures on Windows (replaced atomic temp+rename with direct write)
- `findWrapperEnd` truncating PowerShell wrappers (now searches for column-0 closing brace)
- Tooltip flicker when moving between child elements in web dashboard
- Statusline broken on Windows: backslash paths break Git Bash stdin pipe
- Account name mismatch when CLAUDE_CONFIG_DIR env var overrides default
- Path comparison failures on Windows (case sensitivity + separator normalization)
- `AddAccount` silently wiping email/alias metadata on update
- PowerShell wrapper infinite recursion (now embeds absolute path)

### Changed
- **Breaking**: all `runXxx()` functions replaced by `xxxCmd.Run()` — commands are structs, not functions
- **Breaking**: `command` struct no longer has `fn func()` field; `legacyCmd` wrapper removed
- Shared dirs now include `skills/` and `projects/` (cross-account session resume)
- `parseFlags()` replaces manual `os.Args` iteration in all commands
- `fmt.Fprint*` excluded from errcheck in golangci-lint (CLI stdout output)
- Renamed "primary" to "main" across entire codebase
- Other-account statusline line is now opt-in (default off)
- UI labels centralized in `format/labels.go`
- Unused code removed; `interface{}` modernized to `any`
