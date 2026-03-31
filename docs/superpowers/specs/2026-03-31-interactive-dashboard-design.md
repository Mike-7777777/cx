# Interactive TUI Dashboard — Design Spec

## Goal

Transform `cx dashboard` from a passive auto-refresh display into an interactive TUI with keyboard navigation, section sub-views, and in-place actions (resume session, switch account).

## Constraints

- Zero new dependencies (only `natefinch/atomic` remains)
- Raw terminal mode via syscall (build-tagged Unix/Windows)
- Box width stays at 64 columns
- No mouse support, no terminal resize detection

## Architecture

Two-layer view model:

```
Main View                     Sub View
┌──────────────────────┐      ┌──────────────────────┐
│ cx dashboard         │      │ ACTIVE SESSIONS      │
│──────────────────────│      │──────────────────────│
│▸ ACCOUNTS            │─Enter─▸ session-abc homebase│
│  TODAY'S USAGE       │      │▸ session-def cx-repo │
│  THIS WEEK           │      │  session-ghi papers  │
│  SESSIONS            │      │──────────────────────│
│  INSIGHTS            │      │ r=resume  q=back     │
│  ROI                 │      └──────────────────────┘
│──────────────────────│
│ ↑↓=nav Enter=open    │
│ q=quit               │
└──────────────────────┘
```

**Main View**: Arrow keys move highlight across 6 sections. Each section shows its current summary (same as today). Enter opens sub-view. `q` quits.

**Sub View**: Full-screen detail for the selected section. Sections with selectable rows (SESSIONS, ACCOUNTS) support arrow keys + action keys. Read-only sections (USAGE, WEEK, INSIGHTS, ROI) show expanded detail. `q`/Esc returns to main view.

## Sections

| Section | Main View Summary | Sub View Detail | Actions |
|---------|------------------|-----------------|---------|
| ACCOUNTS | Per-account 5h/7d bar + recommendation | Same + per-account model token distribution | `s` on selected → exec `cx switch` via shell hint |
| TODAY'S USAGE | Tokens, cost, messages, model % | Hourly bar chart + model breakdown table | None |
| THIS WEEK | 7-day bar chart | 30-day calendar heatmap (cost per day, color-coded) | None |
| SESSIONS | Active session count | Full session list (project, age, topic snippet) | `r` on selected → exec `claude --resume <id>` |
| INSIGHTS | Peak hours, cache ratio | Hour-of-day distribution + model cost pie + efficiency metrics | None |
| ROI | Single-line savings | Monthly breakdown + cumulative savings | None |

## Keyboard Input Layer

New files in `internal/platform/`:
- `terminal_unix.go` — raw mode via `syscall.SYS_IOCTL` / termios
- `terminal_windows.go` — raw mode via `SetConsoleMode`

Exported API:
```go
func EnableRawMode() (restore func(), err error)
func ReadKey() (Key, error)  // blocks until keypress
type Key int
const (
    KeyUp Key = iota
    KeyDown
    KeyEnter
    KeyEsc
    KeyQ
    KeyR
    KeyS
    KeyRune // printable character, not handled
)
```

Escape sequences: `\x1b[A` = Up, `\x1b[B` = Down, `\x1b[C` = Right (unused), `\x1b[D` = Left (unused).

## State Machine

```go
type dashState struct {
    view       viewMode    // viewMain or viewSub
    section    int         // 0-5, which section is highlighted
    subCursor  int         // cursor within sub-view (for selectable rows)
    data       *dashboardData
}
```

## Main Loop

```
restore := EnableRawMode()
defer restore()

keys := make(chan Key)
go func() { for { k, _ := ReadKey(); keys <- k } }()

ticker := time.NewTicker(interval)
render(state)

for {
    select {
    case <-ticker.C:
        state.data = scanDashboardData()
        render(state)
    case key := <-keys:
        handleKey(state, key)
        render(state)
    case <-ctx.Done():
        return
    }
}
```

## Rendering

Each view mode has its own render function:
- `renderMainView(state)` — box with 6 section summaries, highlight on `state.section`
- `renderSubView(state)` — full-screen detail for `state.section`, cursor on `state.subCursor`

Footer bar always shows available keybindings for the current view.

## Actions

- **`r` in SESSIONS sub-view**: Restore terminal, exec `claude --resume <sessionID>` (replaces process).
- **`s` in ACCOUNTS sub-view**: Restore terminal, print switch instruction, exit. (Cannot modify parent shell env from a child process — print the command for the user to run.)

## Not Doing

- Mouse support
- Terminal resize detection
- Scrolling (truncate if content exceeds screen height)
- Color theme configuration
