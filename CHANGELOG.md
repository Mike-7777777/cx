# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- `cx config` command: show/primary/rename/set email|alias
- Auto-read identity from `.claude.json` (email, displayName, tier, CC version, session count)
- `cx status` shows subscription tier column and primary marker
- `cx doctor` auto-fixes backslash paths in statusline config
- Graceful statusline fallback when CC doesn't pipe stdin data
- GoReleaser config for Homebrew tap and Scoop bucket
- golangci-lint configuration and CI lint job
- CONTRIBUTING.md, SECURITY.md, Makefile

### Fixed
- Statusline broken on Windows: backslash paths break Git Bash stdin pipe
- Account name mismatch when CLAUDE_CONFIG_DIR env var overrides default
- Path comparison failures on Windows (case sensitivity + separator normalization)
- `AddAccount` silently wiping email/alias metadata on update
- Case-sensitive tier matching in credential reader
- PowerShell wrapper infinite recursion (now embeds absolute path)
- Shell wrapper not updating on re-run of `cx setup`

### Changed
- `ResolveConfigDir` uses `DefaultConfigDir` for empty config_dir (ignores env override)
- `DetectConfigDir` delegates to `DefaultConfigDir` (DRY)
- Consolidated `readAccountInfo` replaces separate credential/identity readers
- Unused `fallbackScores` and `writeJSONError` removed
- `interface{}` modernized to `any`
- Unused HTTP handler parameters replaced with `_`
