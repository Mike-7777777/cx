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

**1. Initialize config**

```bash
cc-monitor init
```

**2. Log in to one or more accounts**

```bash
cc-monitor init --account work
cc-monitor init --account personal
```

**3. Add the shell wrapper** (see Shell Wrapper section below)

**4. Configure your statusline** to call `cc-monitor statusline`

**5. Switch accounts or check status**

```bash
cc-monitor switch work
cc-monitor status
```

## Shell Setup

The `cc switch` wrapper lets you switch accounts with `eval` so the env var is set in the current shell.

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

## Features

| Command       | Description                                      | Latency  |
|---------------|--------------------------------------------------|----------|
| `statusline`  | Emit a compact rate-limit string for statuslines | ~16 ms   |
| `init`        | Initialize config and account credentials        |          |
| `switch`      | Switch the active Claude Code account            |          |
| `sync`        | Sync usage data from Claude API                  |          |
| `status`      | Print account and rate-limit status              |          |

## Shell Wrapper

The wrapper ensures `claude` always runs under the currently active account.

**bash / zsh** — add to `~/.bashrc` or `~/.zshrc`:

```bash
claude() {
  eval "$(cc-monitor env)"
  command claude "$@"
}
```

**fish** — add to `~/.config/fish/config.fish`:

```fish
function claude
    eval (cc-monitor env)
    command claude $argv
end
```

**PowerShell** — add to your `$PROFILE`:

```powershell
function claude {
    Invoke-Expression (cc-monitor env)
    & claude.exe @args
}
```

## Cross-Platform

Pre-built binaries are available for Linux, macOS, and Windows on both amd64 and arm64.

## License

MIT — see [LICENSE](LICENSE).
