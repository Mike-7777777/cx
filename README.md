<p align="center">
  <h1 align="center">cx</h1>
  <p align="center">
    Manage multiple Claude Code accounts from a single binary.<br>
    Switch, sync, monitor rate limits, analyze usage — all in one tool.
  </p>
  <p align="center">
    <a href="https://github.com/Mike-7777777/cx/actions/workflows/ci.yml"><img src="https://github.com/Mike-7777777/cx/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
    <a href="https://go.dev"><img src="https://img.shields.io/badge/Go-1.24-00ADD8?logo=go" alt="Go"></a>
    <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
    <a href="https://github.com/Mike-7777777/cx/releases"><img src="https://img.shields.io/github/v/release/Mike-7777777/cx" alt="Release"></a>
  </p>
</p>

---

```
$ cx status

Account     Auth    5h Usage        Resets In     7d Usage        Note
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
20x         OK      ██░░░  17%      1h 58m        ███░░  62%
5x          OK      ████░  71%      48m           ██░░░  45%

Recommended: 20x (lowest 5h usage at 17%)
```

## Why cx?

You have two Claude Max accounts. You want to use whichever has more quota left. Today you `claude auth logout` / `claude auth login` every time — losing session context, re-entering credentials, wasting minutes.

cx fixes this. Both accounts stay logged in. Switch in one command. Or let the tool auto-select the best one.

<details>
<summary><b>How it compares to ccusage and Claude HUD</b></summary>

cx fills a gap that existing tools don't cover: **multi-account management + lightweight monitoring in a single binary**.

|                          | cx                   | ccusage (11.9k stars)         | Claude HUD (12.6k stars) |
|--------------------------|----------------------|-------------------------------|--------------------------|
| Multi-account switching  | Built-in (setup/switch/sync) | No                     | No                       |
| Cross-account rate limits | Statusline + status table | No                      | No                       |
| Config sync              | Built-in + conflict detection | No                   | No                       |
| Statusline performance   | ~16ms, ~5MB RAM      | 30s+ to stabilize, 300%+ CPU, 1.5-2.4GB RAM ([#804](https://github.com/ryoppippi/ccusage/issues/804)) | ~60-100ms (Node.js) |
| Runtime dependency       | None (single binary) | Node.js / Bun                 | Node.js / Bun            |
| Usage reports            | daily/weekly/monthly/session/blocks/messages | daily/weekly/monthly/session/blocks | None |
| MCP Server               | No                   | Yes                           | No                       |
| Pricing data             | Static (manual update) | LiteLLM (auto-fetch)        | N/A                      |
| Multi-CLI support        | Claude Code only     | 5 CLIs (Claude Code, Codex, Amp, OpenCode, Pi-agent) | Claude Code only |
| Incremental caching      | Yes (0.2s warm)      | No (output cache for statusline only) | N/A              |

**Choose cx if** you manage multiple Claude Code accounts and want one tool for switching, syncing, and monitoring.

**Choose ccusage if** you need the richest usage analytics, MCP integration, or support for non-Claude CLIs.

</details>

## Install

```bash
go install github.com/Mike-7777777/cx@latest
```

Or download a pre-built binary from [Releases](https://github.com/Mike-7777777/cx/releases).

## Quick Start

```bash
# One command sets up everything:
cx setup
```

The interactive setup will:
1. Detect your primary account
2. Create secondary accounts and log them in (browser opens once per account)
3. Install the `cx` shell wrapper for your terminal
4. Run a health check to verify everything works

After setup, you're done:

```bash
cx switch 5x          # switch to 5x account
cx switch 20x         # switch back
cx status             # see all accounts (auth + rate limits)
cx run                # auto-select best account + launch claude
```

<details>
<summary><b>Manual setup (without interactive wizard)</b></summary>

```bash
# 1. Create a secondary account
cx init 5x

# 2. The init command auto-launches login (browser opens once)

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
| `switch <name>` | Switch account in current shell (via wrapper: `cx switch 5x`) |
| `run [-- claude-args]` | Auto-select best account and launch `claude` |
| `run --prefer <name>` | Prefer a specific account (fall back if >80% usage) |
| `run --balance` | Round-robin across accounts for max throughput |
| `status` | All accounts: auth status, rate limits, routing recommendation |

### Monitoring

| Command | Description |
|---------|-------------|
| `dashboard [--interval N]` | Live TUI dashboard (accounts, usage, sessions) |
| `web [--port N]` | Browser dashboard on localhost (dark theme, charts) |
| `usage <mode>` | Usage analysis: `daily`, `weekly`, `monthly`, `session`, `blocks`, `messages` |

### Maintenance

| Command | Description |
|---------|-------------|
| `setup` | Interactive first-time setup |
| `doctor` | Health check all accounts |
| `sync [--force]` | Sync config from primary to all secondaries |
| `login [name]` | Re-authenticate an account (rarely needed) |
| `init <name>` | Create a new account directory |
| `watch` | Auto-sync config changes (30s daemon) |

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

cx uses Claude Code's `CLAUDE_CONFIG_DIR` environment variable to isolate accounts. Each account gets its own directory with separate credentials and session history, but shares plugins and configuration via junctions (Windows) or symlinks (Unix).

```
~/.claude/          <-- primary account (20x)
~/.claude-5x/       <-- secondary account (5x)
    plugins/cache/  --> junction to ~/.claude/plugins/cache/  (shared)
    settings.json   <-- synced from primary
    .credentials.json  <-- independent (separate login)
```

Both accounts stay logged in simultaneously. No re-authentication needed when switching.

## Antivirus Notice

Some antivirus software (ESET, Windows Defender, Kaspersky) may flag the binary as a false positive. This is a [known issue with Go binaries](https://go.dev/doc/faq#virus) — the static linking pattern triggers heuristic signatures shared with Go-based security tools.

cx contains no networking, scanning, or exploit code. It only reads local JSON/JSONL files and writes cache files. You can verify the source code in this repository.

If flagged, add the binary to your antivirus exclusion list.

## Cross-Platform

Pre-built binaries are available for Linux, macOS, and Windows on both amd64 and arm64. Uses NTFS junctions on Windows and symlinks on Unix for shared plugin directories.

## License

MIT — see [LICENSE](LICENSE).
