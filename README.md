<p align="center">
  <h1 align="center">cx</h1>
  <p align="center">
    Manage multiple Claude Code accounts from a single binary.<br>
    Switch, sync, monitor rate limits, analyze usage — all in one tool.
  </p>
  <p align="center">
    <a href="https://github.com/Mike-7777777/cx/actions/workflows/ci.yml"><img src="https://github.com/Mike-7777777/cx/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
    <a href="https://goreportcard.com/report/github.com/Mike-7777777/cx"><img src="https://goreportcard.com/badge/github.com/Mike-7777777/cx" alt="Go Report Card"></a>
    <a href="https://go.dev"><img src="https://img.shields.io/badge/Go-1.24-00ADD8?logo=go" alt="Go"></a>
    <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
    <a href="https://github.com/Mike-7777777/cx/releases"><img src="https://img.shields.io/github/v/release/Mike-7777777/cx" alt="Release"></a>
  </p>
</p>

---

```
$ cx status

  Account     Tier      Auth    5h Usage    5h Reset    7d Usage    7d Reset      Note
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  work        Max 5x    OK      ░░░░░  0%   reset       █░░░░ 15%   Tue 31 19:00
★ personal    Max 20x   OK      █░░░░ 24%   3h04m       ███░░ 67%   Sat 28 02:00

✓ Recommended: work (lowest 5h usage at 0%)

$ cx statusline (shown at bottom of Claude Code)
[work] [Opus 4.6 (1M context)] 43% ctx | $77.67 | 5h: █░░░░ 17% (3h12m) | 7d: ░░░░░ 8% (Tue 31 19:00) ~19%/d
```

## Why cx?

You have two Claude Max accounts. You want to use whichever has more quota left. Today you `claude auth logout` / `claude auth login` every time — losing session context, re-entering credentials, wasting minutes.

cx fixes this. Both accounts stay logged in. Switch in one command. Or let the tool auto-select the best one.

## Features

- **Instant account switching** — `cx switch 5x` sets `CLAUDE_CONFIG_DIR` in your shell. No re-auth needed.
- **Smart auto-routing** — `cx run` picks the account with the lowest 5h usage, factoring in reset proximity.
- **Cross-account session resume** — `cx resume` shows all sessions with first/last message context. Resume any session on any account.
- **Real-time statusline** — Rate limits, cost, context % in Claude Code's status bar. 16ms latency, ~5MB RAM.
- **Live TUI dashboard** — `cx dashboard` renders accounts, weekly chart, active sessions, and ROI in a box-drawn terminal UI with auto-refresh.
- **Usage analytics** — Daily, weekly, monthly, per-session, per-model cost breakdowns. Export as JSON, CSV, or Markdown.
- **Claude Code skill** — Use `/cx` inside Claude Code to check status, usage, and diagnostics without leaving the conversation.
- **Config sync** — Shared settings, plugins, skills, and projects across accounts via junctions/symlinks.
- **Zero dependencies** — Single static binary. No Node.js, no Python, no runtime requirements.

<details>
<summary><b>How it compares to ccusage and Claude HUD</b></summary>

|                          | cx                   | ccusage (11.9k stars)         | Claude HUD (12.6k stars) |
|--------------------------|----------------------|-------------------------------|--------------------------|
| Multi-account switching  | Built-in (setup/switch/sync) | No                     | No                       |
| Cross-account rate limits | Statusline + status table | No                      | No                       |
| Cross-account session resume | Yes (any session on any account) | No              | No                       |
| Config sync              | Built-in + conflict detection | No                   | No                       |
| Statusline performance   | ~16ms, ~5MB RAM      | 30s+ to stabilize, 300%+ CPU, 1.5-2.4GB RAM | ~60-100ms (Node.js) |
| Runtime dependency       | None (single binary) | Node.js / Bun                 | Node.js / Bun            |
| Usage reports            | daily/weekly/monthly/session/blocks/messages | daily/weekly/monthly/session/blocks | None |
| CC Skill integration     | Yes (/cx inside CC)  | No                            | No                       |
| Incremental caching      | Yes (0.2s warm)      | No (output cache only)        | N/A                      |

**Choose cx if** you manage multiple Claude Code accounts and want one tool for switching, syncing, and monitoring.

**Choose ccusage if** you need the richest usage analytics, MCP integration, or support for non-Claude CLIs.

</details>

## Install

```bash
# Go (recommended)
go install github.com/Mike-7777777/cx@latest

# macOS
brew install Mike-7777777/tap/cx

# Windows
scoop bucket add cx https://github.com/Mike-7777777/scoop-bucket
scoop install cx
```

Or download a pre-built binary from [Releases](https://github.com/Mike-7777777/cx/releases).

## Quick Start

```bash
# One command sets up everything:
cx setup
```

The interactive setup will:
1. Detect and register your main account
2. Create secondary accounts and log them in
3. Install the `cx` shell wrapper (PowerShell / Bash / Zsh / Fish)
4. Add `cx` to PATH (for Claude Code and non-interactive shells)
5. Configure the CC statusline integration
6. Install the Claude Code skill (`/cx` inside CC)
7. Run a health check to verify everything works

After setup:

```bash
cx status             # rate limits + routing recommendation
cx switch 5x          # switch to 5x account
cx run                # auto-select best account + launch claude
cx run --yolo         # auto-select + skip permissions
cx resume             # resume a session (shows topic + account picker)
```

<details>
<summary><b>Manual setup (without interactive wizard)</b></summary>

```bash
# 1. Create a secondary account
cx config add 5x

# 2. The command auto-launches login (browser opens once)

# 3. Add shell wrapper — see Shell Setup below

# 4. Done.
```

</details>

<details>
<summary><b>Shell Setup (if not using cx setup)</b></summary>

The `cx` wrapper lets you switch accounts so the `CLAUDE_CONFIG_DIR` env var persists in the current shell.

### Bash / Zsh

Add to `~/.bashrc` or `~/.zshrc`:

```bash
cx() {
    case "$1" in
        switch) eval "$(command cx switch "$2")" ;;
        *) command cx "$@" ;;
    esac
}
```

### Fish

Add to `~/.config/fish/functions/cx.fish`:

```fish
function cx
    if test "$argv[1]" = "switch"
        eval (command cx switch $argv[2])
    else
        command cx $argv
    end
end
```

### PowerShell

Add to your `$PROFILE`:

```powershell
function cx {
    if ($args[0] -eq "switch") {
        $cmd = & cx switch $args[1] --shell=powershell
        Invoke-Expression ($cmd -join "`n")
    } else {
        & cx @args
    }
}
```

### Tab completion

```bash
# Bash
eval "$(cx completion bash)"

# Fish
cx completion fish | source

# PowerShell
cx completion powershell | Out-String | Invoke-Expression
```

</details>

## Command Reference

### Daily Use

| Command | Description |
|---------|-------------|
| `switch <name>` | Switch account in current shell |
| `run [-- claude-args]` | Auto-select best account and launch `claude` |
| `run --prefer <name>` | Prefer a specific account (fall back if >80% usage) |
| `run --balance` | Round-robin across accounts for max throughput |
| `status` | All accounts: auth, tier, rate limits, recommendation |
| `sessions [--all]` | List recent sessions with first/last message topic |
| `resume [term\|--last]` | Resume a session by fuzzy match or picker |
| `resume --on <acct>` | Resume a session on a specific account (cross-account) |
| `config` | Show full config: email, tier, CC version, session count |
| `config add <name>` | Add a new account (creates dir, links, syncs, launches login) |

### Monitoring

| Command | Description |
|---------|-------------|
| `daemon` | Combined auto-switch + config-sync daemon |
| `dashboard [--interval N]` | Live TUI dashboard (accounts, usage, sessions, ROI) |
| `usage <mode>` | Usage analysis: `daily`, `weekly`, `monthly`, `session`, `blocks`, `messages` |
| `insights [--all]` | Usage pattern analysis (hourly heatmap, model distribution) |
| `predict [--json]` | Forecast rate limit exhaustion |

### Maintenance

| Command | Description |
|---------|-------------|
| `setup` | Interactive first-time setup |
| `doctor` | Health check all accounts (auto-fixes common issues) |
| `sync [--force]` | Sync config from main to all secondaries |
| `login [name]` | Re-authenticate an account |

<details>
<summary><b>Usage flags</b></summary>

| Flag | Description |
|------|-------------|
| `--json` | JSON output |
| `--format csv\|md` | CSV or Markdown export |
| `--since YYYY-MM-DD` | Filter by date |
| `--account <name>` | Specific account only |
| `--all` | Merge all accounts |
| `--breakdown` | Per-model subtotals |
| `--by-project` | Group by project directory |
| `--compare` | Trend vs previous period |
| `--subagents` | Main vs subagent cost split |
| `--roi` | ROI: subscription cost vs equivalent API cost |
| `--all-tools` | Include Codex/Amp/OpenCode data |
| `--no-cache` | Force full rescan (skip incremental cache) |

</details>

## How It Works

cx uses Claude Code's `CLAUDE_CONFIG_DIR` environment variable to isolate accounts. Each account gets its own directory with separate credentials, but shares plugins, skills, projects, and configuration via junctions (Windows) or symlinks (Unix).

```
~/.claude/              <-- main account (personal, Max 20x)
    .credentials.json   <-- independent (separate login)
    rate-cache.json     <-- rate limit data (written by cx statusline)
    settings.json       <-- synced to secondaries
    plugins/cache/      <-- shared via junction/symlink
    skills/             <-- shared (cx skill installed here)
    projects/           <-- shared (enables cross-account session resume)

~/.claude-work/         <-- secondary account (work, Max 5x)
    .credentials.json   <-- independent
    plugins/cache/  --> junction to ~/.claude/plugins/cache/
    skills/         --> junction to ~/.claude/skills/
    projects/       --> junction to ~/.claude/projects/
```

Both accounts stay logged in simultaneously. `cx switch` only changes an environment variable — no re-authentication needed.

## Architecture

cx is a single Go binary (~12k lines) with a clean command architecture:

- **Runner pattern** — every command implements `Run(ctx, app, args) error`, enabling dependency injection and unit testing
- **App DI container** — shared `Registry`, `io.Writer`, and color flag injected into every command
- **context.Context** — threaded through all commands for graceful shutdown
- **Internal packages** — `config`, `cache`, `format`, `usage`, `statusline`, `platform`, `errors`
- **125+ tests** across 8 packages with table-driven patterns

Single external dependency: [`natefinch/atomic`](https://github.com/natefinch/atomic) for atomic file writes.

## Antivirus Notice

Some antivirus software (ESET, Windows Defender, Kaspersky) may flag the binary as a false positive. This is a [known issue with Go binaries](https://go.dev/doc/faq#virus) — the static linking pattern triggers heuristic signatures.

cx contains no networking, scanning, or exploit code. It only reads local JSON/JSONL files and writes cache files. You can verify the source code in this repository.

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

```bash
# Dev setup
git clone https://github.com/Mike-7777777/cx.git
cd cx
go build -o cx .
go test ./... -count=1
golangci-lint run
```

## License

MIT — see [LICENSE](LICENSE).
