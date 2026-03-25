# cc-monitor

A single Go binary for managing multiple Claude Code accounts with real-time rate limit monitoring.

## Why not ccusage or Claude HUD?

ccusage statusline consumes 300%+ CPU and 1.5GB+ RAM (#804). Claude HUD is 4K lines TypeScript that
can't coexist with custom statusline logic. Neither supports multi-account. cc-monitor is a single
~2MB Go binary that replaces both.

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
