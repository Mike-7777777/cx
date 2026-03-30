---
name: cx
description: Use when the user wants to check Claude Code account status, rate limits, usage stats, session history, or manage multi-account operations. Triggers on keywords like "quota", "rate limit", "usage", "account status", "cx", "which account", "how much left", "spending".
---

## cx — Claude Code Multi-Account Manager

cx manages multiple Claude Code Max Plan accounts from a single machine. It monitors rate limits, tracks spending, and provides usage analytics.

### Command Map

| Intent | Command | Notes |
|--------|---------|-------|
| Account status + rate limits | `cx status` | Auth state, 5h/7d usage%, reset times |
| Daily/weekly/monthly usage | `cx usage <daily\|weekly\|monthly>` | Cost breakdown by period |
| Per-session usage | `cx usage session` | Tokens + cost per session |
| Usage as JSON (for analysis) | `cx usage daily --format json` | Machine-readable for deeper analysis |
| Per-block (5h) usage | `cx usage blocks` | Aggregated by 5-hour rate limit windows |
| Session history | `cx sessions --all` | All sessions across accounts |
| Health check | `cx doctor` | Diagnose config/auth issues |
| Sync config across accounts | `cx sync` | Push main account config to secondaries |
| Re-authenticate | `cx login <name>` | Fix expired OAuth tokens |
| Usage pattern analysis | `cx insights [--all]` | Hourly heatmap, model distribution, efficiency metrics |
| Predict rate limit exhaustion | `cx predict` | Velocity, time-to-exhaust, actionable warnings |
| Auto-switch + sync daemon | `cx daemon [--no-sync] [--no-switch]` | Combined auto-switch + config-sync |

### Workflow

1. **Run command**: Use the `Bash` tool with the commands above
2. **Interpret output**: Summarize findings, highlight actionable insights
3. **Combine when useful**: For a general status check, run `status` + `usage daily` in parallel

### Example Patterns

- "how much quota left" / "rate limit" → `cx status`
- "spending this week" / "usage" → `cx usage weekly`
- "check cx" / "account status" → `cx status` + `cx usage daily` (parallel)
- "account issues" / "auth error" → `cx doctor`
- "recent sessions" → `cx sessions --all`
- "usage patterns" / "peak hours" → `cx insights --all`
- "will I hit the limit" / "predict" → `cx predict`

### Composite Analysis

When the user asks broad questions, combine multiple commands and provide insights:

```bash
# Run in parallel
cx status
cx usage daily
```

Then analyze:
- Which account has more remaining quota
- Spending trend (is today unusually high?)
- Time until rate limit resets
- Recommendation: which account to prefer

### Limitations

- **`cx switch`**: Cannot work here — requires `eval` to modify shell environment. Tell user to run `cx switch <name>` in their terminal.
- **`cx run`**: Not applicable — user is already inside Claude Code.
- **`cx dashboard` / `cx web`**: Interactive/persistent processes, not suitable for skill invocation.
- **`cx statusline`**: Already running in CC's status bar — no need to invoke manually.
