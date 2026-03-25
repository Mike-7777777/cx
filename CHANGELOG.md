# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- `cx sessions` command: list recent CC sessions across all accounts
- `cx resume` command: resume a session by fuzzy match, picker, or `--last`
- `cx config` command: show/main/rename/set email|alias
- Auto-read identity from `.claude.json` (email, displayName, tier, CC version, session count)
- `cx status` shows subscription tier column, main marker, and 7d reset date
- 7d daily budget indicator in statusline: `~14%/d` (green/yellow/red)
- `cx doctor` auto-fixes stale statusline paths and project-level overrides
- Graceful statusline fallback when CC doesn't pipe stdin data
- Smart statusline path: uses `cx statusline` if in PATH, else absolute path
- GoReleaser config for Homebrew tap and Scoop bucket
- golangci-lint configuration and CI lint job
- CONTRIBUTING.md, SECURITY.md, CHANGELOG.md, Makefile

### Fixed
- Statusline broken on Windows: backslash paths break Git Bash stdin pipe
- Account name mismatch when CLAUDE_CONFIG_DIR env var overrides default
- Path comparison failures on Windows (case sensitivity + separator normalization)
- `AddAccount` silently wiping email/alias metadata on update
- Case-sensitive tier matching in credential reader
- PowerShell wrapper infinite recursion (now embeds absolute path)
- Shell wrapper not updating on re-run of `cx setup`

### Changed
- Renamed "primary" to "main" across entire codebase (CLI, JSON, docs)
- Other-account statusline line is now opt-in (default off)
- UI labels centralized in `format/labels.go`
- `ResolveConfigDir` uses `DefaultConfigDir` for empty config_dir (ignores env override)
- `DetectConfigDir` delegates to `DefaultConfigDir` (DRY)
- Consolidated `readAccountInfo` replaces separate credential/identity readers
- Unused code removed (`fallbackScores`, `writeJSONError`)
- `interface{}` modernized to `any`
- Unused HTTP handler parameters replaced with `_`
