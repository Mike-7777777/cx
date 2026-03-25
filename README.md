# cc-monitor

[![CI](https://github.com/Mike-7777777/cc-monitor/actions/workflows/ci.yml/badge.svg)](https://github.com/Mike-7777777/cc-monitor/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A single Go binary for managing multiple Claude Code accounts with real-time rate limit monitoring.

## How it compares

cc-monitor fills a gap that existing tools don't cover: **multi-account management + lightweight monitoring in a single binary**.

|                          | cc-monitor           | ccusage (11.9k stars)         | Claude HUD (12.6k stars) |
|--------------------------|----------------------|-------------------------------|--------------------------|
| Multi-account switching  | Built-in (init/switch/sync) | No                      | No                       |
| Cross-account rate limits | Statusline + status table | No                      | No                       |
| Config sync              | Built-in + conflict detection | No                   | No                       |
| Statusline performance   | ~16ms, ~5MB RAM      | 30s+ to stabilize, 300%+ CPU, 1.5-2.4GB RAM ([#804](https://github.com/ryoppippi/ccusage/issues/804)) | ~60-100ms (Node.js) |
| Runtime dependency       | None (single binary) | Node.js / Bun                 | Node.js / Bun            |
| Usage reports            | daily/session/blocks | daily/weekly/monthly/session/blocks | None              |
| MCP Server               | No                   | Yes                           | No                       |
| Pricing data             | Static (manual update) | LiteLLM (auto-fetch)        | N/A                      |
| Multi-CLI support        | Claude Code only     | 5 CLIs (Claude Code, Codex, Amp, OpenCode, Pi-agent) | Claude Code only |
| Incremental caching      | Yes (0.2s warm)      | No (output cache for statusline only) | N/A              |

**Choose cc-monitor if** you manage multiple Claude Code accounts and want one tool for switching, syncing, and monitoring.

**Choose ccusage if** you need the richest usage analytics, MCP integration, or support for non-Claude CLIs.

## Install

```bash
go install github.com/Mike-7777777/cc-monitor@latest
```

Or download a pre-built binary from [Releases](https://github.com/Mike-7777777/cc-monitor/releases).

## Quick Start

**1. Initialize a secondary account**

Your default Claude Code config (`~/.claude/`) is the primary account. Use `init` to create additional accounts:

```bash
cc-monitor init 5x
cc-monitor init personal
```

This creates `~/.claude-5x/` (or `~/.claude-personal/`), links shared directories, and syncs config files from the primary.

**2. Log in to the new account**

```bash
CLAUDE_CONFIG_DIR=~/.claude-5x claude auth login
```

**3. Add the shell wrapper** (see Shell Setup below)

**4. Switch accounts and check status**

```bash
cc switch 5x          # via shell wrapper (sets env in current shell)
cc-monitor status     # show all accounts with rate limits
cc-monitor usage      # analyze token usage and costs
```

## Shell Setup

The `cc` wrapper lets you switch accounts so the `CLAUDE_CONFIG_DIR` env var is set in the current shell.

### Bash / Zsh

Add to `~/.bashrc` or `~/.zshrc`:

```bash
cc() {
    case "$1" in
        switch) eval "$(cc-monitor switch "$2")" ;;
        *) cc-monitor "$@" ;;
    esac
}
```

### Fish

Add to `~/.config/fish/functions/cc.fish`:

```fish
function cc
    if test "$argv[1]" = "switch"
        eval (cc-monitor switch $argv[2])
    else
        cc-monitor $argv
    end
end
```

### PowerShell

Add to your `$PROFILE`:

```powershell
function cc {
    if ($args[0] -eq "switch") {
        Invoke-Expression (cc-monitor switch $args[1])
    } else {
        & cc-monitor @args
    }
}
```

## Commands

| Command       | Description                                                | Latency  |
|---------------|------------------------------------------------------------|----------|
| `statusline`  | Emit a compact rate-limit string for tmux / shell prompts  | ~16 ms   |
| `init <name>` | Create a secondary account directory and register it       |          |
| `switch <name>` | Emit shell commands to switch the active account (use with `eval`) | |
| `sync`        | Sync config files from the primary account to all secondaries |       |
| `status`      | Print all accounts with rate-limit bars and recommendations |         |
| `usage [daily\|session\|blocks]` | Analyze token usage and costs (incremental cache) | |
| `version`     | Print version information                                  |          |
| `help`        | Show help message                                          |          |

## Statusline Integration

### tmux

```tmux
set -g status-right '#(cc-monitor statusline)'
```

### Starship

```toml
[custom.cc]
command = "cc-monitor statusline"
when = true
```

## Cross-Platform

Pre-built binaries are available for Linux, macOS, and Windows on both amd64 and arm64.

## License

MIT — see [LICENSE](LICENSE).
