# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- `cx resume` now supports the same flag semantics as `cx run`: `--prefer <acct>` / `-pf` (soft selection with rate-limit fallback), `-y` / `--yolo` alias, and pass-through of unknown flags to claude (`--remote-control`, `--model sonnet`, etc.)
- `parseResumeArgs()` extracted as a testable function with full coverage (help, last, on, prefer, yolo, passthrough, `--` separator, mutual exclusion, regression tests)

### Fixed
- `cx resume --rc --yolo --prefer QM` no longer fails with `no session matching "--rc"`. Unknown `--flags` previously fell into the positional search term; they now pass through to claude, and `--prefer` is recognised as a cx flag
- `cx resume` interactive picker no longer silently caps the list at 15 sessions — every collected session is shown, and the collect step no longer caps at 50 either. Narrow large histories with a leading fuzzy term (`cx resume <term>`).

### Changed
- `cx resume` search term must now be the FIRST positional arg. This disambiguates cases like `cx resume --model sonnet` (passes `--model sonnet` to claude) from `cx resume fix-bug --model sonnet` (searches for `fix-bug`, passes `--model sonnet` to claude).
- `--on` and `--prefer` in `cx resume` are mutually exclusive; passing both returns an explicit error instead of silently preferring one.

## [0.5.0] - 2026-04-12

### Added
- **Flag pass-through** — unknown flags (`--remote-control`, `--verbose`, `-p`, etc.) now pass through to claude as-is; cx never needs updating when claude adds new flags
- `-y` shorthand alias for `--dangerously-skip-permissions` in both `cx run` and `cx resume`
- `parseRunArgs()` extracted as testable function with 12 new tests covering alias expansion, pass-through, mixed flags, `--` separator, and edge cases

### Fixed
- `--prefer` without a value now returns a clear error instead of silently passing to claude
- `-y` works in `cx resume` (consistency with `cx run`)
- Stale doc comment in `shortProjectName` (wrong example output)
- Personal path examples replaced with generic `D--projects-myapp`

### Changed
- `runAliases` moved to package-level variable for clarity
- README: Homebrew/Scoop install moved to "coming soon" (repos created but not yet populated)
- CHANGELOG restructured with versioned entries and comparison links
- GitHub Actions dependencies updated (checkout v6, setup-go v6)

## [0.4.0] - 2026-03-31

### Added
- **Interactive TUI dashboard** — keyboard navigation (arrow keys, Enter, q), sub-views for accounts (with switch), sessions (with resume), usage hourly, 30-day heatmap, insights, and ROI
- Raw terminal mode and key reading for Unix and Windows
- INSIGHTS summary section in dashboard (peak hours, cache ratio)
- State machine for dashboard navigation with comprehensive tests

## [0.3.0] - 2026-03-31

### Changed
- Removed web dashboard; TUI dashboard is now the only dashboard mode

### Fixed
- Git hooks use `git diff` for gofmt check (CRLF-safe on Windows)

## [0.2.1] - 2026-03-30

### Fixed
- Pre-commit/pre-push hooks check committed files, not working tree

### Changed
- Eliminated regexp dependency, reducing binary size by 236 KB

## [0.2.0] - 2026-03-30

### Added
- **Runner architecture** — all commands implement `Runner` interface with `Run(ctx, app, args) error`, enabling dependency injection and unit testing
- **App DI container** — `App` struct holds `Registry`, `Stdout`, `Stderr`, `UseColor`; commands never access `os.Args` or `os.Stdout` directly
- **context.Context** — threaded through all commands for graceful shutdown
- `cx predict` — rate limit exhaustion forecasting
- `cx insights` — hourly, model, project, and efficiency analysis
- `cx auto` — monitoring daemon with threshold-based switching
- `cx daemon` — unified daemon combining auto + watch
- `--yolo` flag for `cx resume`
- Effort level badge in statusline from CC settings.json
- 7d headroom-based routing and burn rate indicator in statusline
- Session pagination with load-more in web dashboard
- 100+ new tests across all packages

### Fixed
- Deduplicate sessions from cross-account symlinks
- Harden boolean flag checks, validate `--on` arg, check main config dir
- Cross-account resume via symlinked session files
- Doctor skips main account for junction checks, detects stale copies
- Web dashboard: ROI math, port parsing, ready endpoint, currency format, contrast
- Tooltip mouseleave, findWrapperEnd edge case
- Filter `<synthetic>` model entries from usage parsing
- Route sync log output through `io.Writer` instead of `os.Stderr`

### Changed
- **Breaking**: all `runXxx()` functions replaced by `xxxCmd.Run()` — commands are structs, not functions
- Renamed `init` to `config add` subcommand
- Replaced `auto` + `watch` with unified `daemon` command
- `parseFlags()` replaces manual `os.Args` iteration in all commands
- UI labels centralized in `format/labels.go`
- Renamed "primary" to "main" across entire codebase

## [0.1.0] - 2026-03-26

### Added
- **Multi-account management** — `cx setup`, `cx switch`, `cx config add`
- **Smart routing** — `cx run` picks account with lowest 5h usage; `--prefer`, `--balance` modes
- **Rate limit monitoring** — `cx status` shows all accounts with auth, tier, 5h/7d usage
- **Statusline integration** — real-time rate limits, cost, context % in Claude Code's status bar (~16ms)
- **Usage analytics** — daily, weekly, monthly, per-session, per-model breakdowns with JSON/CSV/Markdown export
- **Config sync** — shared settings, plugins, skills, projects across accounts via junctions/symlinks
- **Cross-account session resume** — `cx sessions` lists all sessions; `cx resume` with topic preview and `--on` flag
- **TUI dashboard** — box-drawn terminal UI with accounts, weekly chart, sessions, and ROI
- **Web dashboard** — embedded HTML with dark theme, charts, and auto-refresh
- **Claude Code skill** — use `/cx` inside CC to check status and diagnostics
- **Shell completion** — Bash, Zsh, Fish, PowerShell
- `cx doctor` — health check with auto-fix for common issues
- `cx login` — re-authenticate accounts
- GoReleaser config for cross-platform binaries (Linux, macOS, Windows × amd64, arm64)
- CI pipeline: gofmt, golangci-lint, govulncheck, 3-platform tests with `-race`
- 125+ tests across 8 packages

[Unreleased]: https://github.com/Mike-7777777/cx/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/Mike-7777777/cx/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/Mike-7777777/cx/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/Mike-7777777/cx/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/Mike-7777777/cx/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/Mike-7777777/cx/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Mike-7777777/cx/releases/tag/v0.1.0
