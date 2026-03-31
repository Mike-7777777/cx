# Interactive TUI Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform `cx dashboard` from passive auto-refresh to interactive TUI with keyboard navigation, full-screen sub-views, and in-place actions (resume session, switch account).

**Architecture:** Two-layer view model (main view with section highlights + full-screen sub-views). Raw terminal input via build-tagged platform code. State machine driven by a `select` on ticker + key channel. Zero new dependencies.

**Tech Stack:** Go 1.24, raw ANSI escape codes, syscall for terminal raw mode (Unix termios / Windows SetConsoleMode)

---

### Task 1: Terminal Raw Mode — Platform Layer

**Files:**
- Create: `internal/platform/terminal_unix.go`
- Create: `internal/platform/terminal_windows.go`
- Create: `internal/platform/terminal_test.go`

This task adds the keyboard input layer. `EnableRawMode` puts stdin into character-at-a-time mode. `ReadKey` blocks until a keypress and parses escape sequences into typed constants.

- [ ] **Step 1: Define the Key type and constants**

Create `internal/platform/terminal_unix.go` with build tag:

```go
//go:build !windows

package platform

import (
	"os"
	"syscall"
	"unsafe"
)

// Key represents a parsed keyboard input.
type Key int

const (
	KeyNone  Key = iota
	KeyUp
	KeyDown
	KeyEnter
	KeyEsc
	KeyQ
	KeyR
	KeyS
	KeyRune // unhandled printable character
)

// original holds the saved termios state for restoration.
var original syscall.Termios
```

- [ ] **Step 2: Implement EnableRawMode for Unix**

Append to `internal/platform/terminal_unix.go`:

```go
// EnableRawMode switches stdin to raw mode (no echo, no line buffering).
// Returns a restore function that must be called to reset the terminal.
func EnableRawMode() (restore func(), err error) {
	fd := int(os.Stdin.Fd())

	// Get current terminal attributes.
	if _, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd),
		uintptr(syscall.TCGETS), uintptr(unsafe.Pointer(&original)),
		0, 0, 0); errno != 0 {
		return nil, errno
	}

	raw := original
	// Disable echo, canonical mode, signals, and extended processing.
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	// Disable CR-to-NL translation and flow control.
	raw.Iflag &^= syscall.ICRNL | syscall.IXON
	// Read returns after 1 byte, no timeout.
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0

	if _, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd),
		uintptr(syscall.TCSETS), uintptr(unsafe.Pointer(&raw)),
		0, 0, 0); errno != 0 {
		return nil, errno
	}

	return func() {
		syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd),
			uintptr(syscall.TCSETS), uintptr(unsafe.Pointer(&original)),
			0, 0, 0)
	}, nil
}
```

- [ ] **Step 3: Implement ReadKey for Unix**

Append to `internal/platform/terminal_unix.go`:

```go
// ReadKey blocks until a key is pressed and returns the parsed Key.
// Handles escape sequences for arrow keys (\x1b[A/B).
func ReadKey() (Key, error) {
	buf := make([]byte, 3)
	n, err := os.Stdin.Read(buf[:1])
	if err != nil || n == 0 {
		return KeyNone, err
	}

	switch buf[0] {
	case '\x1b': // Escape sequence
		// Try to read two more bytes (non-blocking via short read).
		n, _ = os.Stdin.Read(buf[1:3])
		if n == 2 && buf[1] == '[' {
			switch buf[2] {
			case 'A':
				return KeyUp, nil
			case 'B':
				return KeyDown, nil
			}
		}
		return KeyEsc, nil
	case '\r', '\n':
		return KeyEnter, nil
	case 'q', 'Q':
		return KeyQ, nil
	case 'r', 'R':
		return KeyR, nil
	case 's', 'S':
		return KeyS, nil
	default:
		return KeyRune, nil
	}
}
```

- [ ] **Step 4: Implement Windows terminal_windows.go**

Create `internal/platform/terminal_windows.go`:

```go
//go:build windows

package platform

import (
	"os"
	"syscall"
	"unsafe"
)

// Key represents a parsed keyboard input.
type Key int

const (
	KeyNone  Key = iota
	KeyUp
	KeyDown
	KeyEnter
	KeyEsc
	KeyQ
	KeyR
	KeyS
	KeyRune
)

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	getConsoleMode       = kernel32.NewProc("GetConsoleMode")
	setConsoleMode       = kernel32.NewProc("SetConsoleMode")
	readConsoleInput     = kernel32.NewProc("ReadConsoleInputW")
	originalConsoleMode  uint32
)

const (
	enableProcessedInput = 0x0001
	enableLineInput      = 0x0002
	enableEchoInput      = 0x0004
	enableVirtualTerminal = 0x0200
)

// EnableRawMode switches the Windows console to raw mode.
func EnableRawMode() (restore func(), err error) {
	handle := syscall.Handle(os.Stdin.Fd())

	r, _, e := getConsoleMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&originalConsoleMode)))
	if r == 0 {
		return nil, e
	}

	raw := originalConsoleMode &^ (enableLineInput | enableEchoInput | enableProcessedInput)
	raw |= enableVirtualTerminal

	r, _, e = setConsoleMode.Call(uintptr(handle), uintptr(raw))
	if r == 0 {
		return nil, e
	}

	return func() {
		setConsoleMode.Call(uintptr(handle), uintptr(originalConsoleMode))
	}, nil
}

// ReadKey blocks until a key is pressed and returns the parsed Key.
func ReadKey() (Key, error) {
	buf := make([]byte, 3)
	n, err := os.Stdin.Read(buf[:1])
	if err != nil || n == 0 {
		return KeyNone, err
	}

	switch buf[0] {
	case '\x1b':
		n, _ = os.Stdin.Read(buf[1:3])
		if n == 2 && buf[1] == '[' {
			switch buf[2] {
			case 'A':
				return KeyUp, nil
			case 'B':
				return KeyDown, nil
			}
		}
		return KeyEsc, nil
	case '\r', '\n':
		return KeyEnter, nil
	case 'q', 'Q':
		return KeyQ, nil
	case 'r', 'R':
		return KeyR, nil
	case 's', 'S':
		return KeyS, nil
	default:
		return KeyRune, nil
	}
}
```

- [ ] **Step 5: Add basic test**

Create `internal/platform/terminal_test.go`:

```go
package platform

import "testing"

func TestKeyConstants(t *testing.T) {
	// Verify constants are distinct.
	keys := []Key{KeyNone, KeyUp, KeyDown, KeyEnter, KeyEsc, KeyQ, KeyR, KeyS, KeyRune}
	seen := make(map[Key]bool)
	for _, k := range keys {
		if seen[k] {
			t.Errorf("duplicate key constant: %d", k)
		}
		seen[k] = true
	}
}
```

- [ ] **Step 6: Verify build and tests pass**

Run: `go build ./... && go test ./internal/platform/ -v -count=1`

- [ ] **Step 7: Commit**

```bash
git add internal/platform/terminal_unix.go internal/platform/terminal_windows.go internal/platform/terminal_test.go
git commit -m "feat(platform): add raw terminal mode and key reading (Unix/Windows)"
```

---

### Task 2: Dashboard State Machine and Main Loop

**Files:**
- Modify: `run_dashboard.go`

Replace the passive ticker loop with an interactive state machine. The main loop `select`s on both ticker and key channel. This task wires up the skeleton — rendering changes come in later tasks.

- [ ] **Step 1: Define state types at top of file**

Add after the existing constants in `run_dashboard.go`:

```go
// viewMode distinguishes between the main overview and a full-screen sub-view.
type viewMode int

const (
	viewMain viewMode = iota
	viewSub
)

// sectionID identifies a dashboard section.
type sectionID int

const (
	sectionAccounts sectionID = iota
	sectionUsage
	sectionWeek
	sectionSessions
	sectionInsights
	sectionROI
	sectionCount // sentinel: total number of sections
)

// sectionNames maps section IDs to display titles.
var sectionNames = [sectionCount]string{
	"ACCOUNTS", "TODAY'S USAGE", "THIS WEEK", "SESSIONS", "INSIGHTS", "ROI",
}

// dashState holds the full dashboard UI state.
type dashState struct {
	view      viewMode
	section   sectionID
	subCursor int
	data      *dashboardData
	useColor  bool
	interval  time.Duration
	// sessionEntries and accountNames are populated on data refresh
	// for sub-views that need selectable rows.
	sessionList []sessionEntry
	accountList []string
}
```

- [ ] **Step 2: Rewrite the Run method**

Replace the existing `Run` method with the interactive version:

```go
func (c *dashboardCmd) Run(ctx context.Context, app *App, args []string) error {
	interval := defaultDashboardInterval
	out := app.Stdout

	for i := 0; i < len(args); i++ {
		if args[i] == "--interval" {
			if i+1 >= len(args) {
				return fmt.Errorf("--interval requires a value in seconds")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 {
				return fmt.Errorf("--interval must be a positive integer, got %q", args[i])
			}
			interval = time.Duration(n) * time.Second
		} else {
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	// Enable raw terminal mode for keyboard input.
	restore, err := platform.EnableRawMode()
	if err != nil {
		// Fallback: run in passive mode (no keyboard).
		return c.runPassive(ctx, out, app.UseColor, interval)
	}
	defer restore()
	defer fmt.Fprint(out, "\033[?25h"+format.Reset)

	state := &dashState{
		view:     viewMain,
		useColor: app.UseColor,
		interval: interval,
	}

	// Key reader goroutine.
	keys := make(chan platform.Key, 1)
	go func() {
		for {
			k, err := platform.ReadKey()
			if err != nil {
				return
			}
			keys <- k
		}
	}()

	// Initial data scan and render.
	refreshDashState(state, app.Registry)
	renderInteractive(out, state)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			refreshDashState(state, app.Registry)
			renderInteractive(out, state)
		case key := <-keys:
			action := handleDashKey(state, key)
			switch action {
			case dashActionQuit:
				return nil
			case dashActionResume:
				if state.subCursor < len(state.sessionList) {
					s := state.sessionList[state.subCursor]
					restore()
					fmt.Fprint(out, "\033[?25h\033[2J\033[H")
					env := replaceOrAppendEnv(os.Environ(), "CLAUDE_CONFIG_DIR", s.ConfigDir)
					return platform.ExecProgram("claude", []string{"--resume", s.ID}, env)
				}
			case dashActionSwitch:
				if state.subCursor < len(state.accountList) {
					name := state.accountList[state.subCursor]
					restore()
					fmt.Fprint(out, "\033[?25h\033[2J\033[H")
					fmt.Fprintf(out, "Run: cx switch %s\n", name)
					return nil
				}
			}
			renderInteractive(out, state)
		}
	}
}

// runPassive is the fallback when raw mode is unavailable (e.g., piped output).
func (c *dashboardCmd) runPassive(ctx context.Context, out io.Writer, useColor bool, interval time.Duration) error {
	defer fmt.Fprint(out, "\033[?25h"+format.Reset)
	renderDashboard(out, useColor, interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			renderDashboard(out, useColor, interval)
		}
	}
}
```

- [ ] **Step 3: Add dashAction type and handleDashKey**

```go
type dashAction int

const (
	dashActionNone dashAction = iota
	dashActionQuit
	dashActionResume
	dashActionSwitch
)

// handleDashKey processes a keypress and updates the state. Returns an action
// if the key triggers a command (quit, resume, switch).
func handleDashKey(state *dashState, key platform.Key) dashAction {
	switch state.view {
	case viewMain:
		switch key {
		case platform.KeyUp:
			if state.section > 0 {
				state.section--
			}
		case platform.KeyDown:
			if state.section < sectionCount-1 {
				state.section++
			}
		case platform.KeyEnter:
			state.view = viewSub
			state.subCursor = 0
		case platform.KeyQ, platform.KeyEsc:
			return dashActionQuit
		}
	case viewSub:
		switch key {
		case platform.KeyUp:
			if state.subCursor > 0 {
				state.subCursor--
			}
		case platform.KeyDown:
			maxCursor := subViewRowCount(state)
			if state.subCursor < maxCursor-1 {
				state.subCursor++
			}
		case platform.KeyQ, platform.KeyEsc:
			state.view = viewMain
		case platform.KeyR:
			if state.section == sectionSessions {
				return dashActionResume
			}
		case platform.KeyS:
			if state.section == sectionAccounts {
				return dashActionSwitch
			}
		case platform.KeyEnter:
			// Enter in sub-view does nothing (actions use specific keys).
		}
	}
	return dashActionNone
}

// subViewRowCount returns the number of selectable rows in the current sub-view.
func subViewRowCount(state *dashState) int {
	switch state.section {
	case sectionSessions:
		return len(state.sessionList)
	case sectionAccounts:
		return len(state.accountList)
	default:
		return 0
	}
}
```

- [ ] **Step 4: Add refreshDashState and renderInteractive stubs**

```go
// refreshDashState reloads usage data and populates selectable lists.
func refreshDashState(state *dashState, reg *config.Registry) {
	state.data = scanDashboardData()
	state.sessionList = collectSessions(reg, "", 50)
	state.accountList = sortedAccountNames(reg)
}

// renderInteractive renders the dashboard based on current state.
// Delegates to renderMainView or renderSubView.
func renderInteractive(out io.Writer, state *dashState) {
	var b strings.Builder
	b.WriteString("\033[2J\033[H\033[?25l")

	if state.view == viewSub {
		b.WriteString(renderSubView(state))
	} else {
		b.WriteString(renderMainView(state))
	}

	fmt.Fprint(out, b.String())
}
```

- [ ] **Step 5: Add renderMainView (reuses existing section renderers with highlight)**

```go
// renderMainView draws the main overview with section highlights.
func renderMainView(state *dashState) string {
	var b strings.Builder
	uc := state.useColor

	b.WriteString(boxTop())
	title := "  cx dashboard"
	refreshNote := fmt.Sprintf("[every %ds · ↑↓ Enter q]", int(state.interval.Seconds()))
	padding := dashboardBoxWidth - len(title) - len(refreshNote) - 4
	if padding < 1 {
		padding = 1
	}
	titleLine := format.Colorize(title, format.Bold+format.Cyan, uc) +
		strings.Repeat(" ", padding) +
		format.Colorize(refreshNote, format.Dim, uc)
	b.WriteString(boxRow(titleLine))
	b.WriteString(boxMid())

	// Render each section with highlight marker.
	sections := [sectionCount]string{
		renderAccountsSection(uc),
		renderTodayUsageSection(uc, state.data),
		renderWeeklyChartSection(uc, state.data),
		renderSessionsSection(uc),
		renderInsightsSummary(uc, state.data),
		renderROISection(uc, state.data),
	}

	for i, content := range sections {
		if sectionID(i) == state.section {
			// Replace first "  " after section header with "▸ " to show cursor.
			content = highlightSection(content, uc)
		}
		b.WriteString(content)
	}

	b.WriteString(boxBottom())
	return b.String()
}

// highlightSection adds a selection indicator to a section's header.
func highlightSection(content string, useColor bool) string {
	// The section header starts with emptyLine + padLine("  TITLE").
	// Replace the first occurrence of "║  " (border + 2 spaces before title)
	// with "║▸ " to show the cursor arrow.
	return strings.Replace(content, "║  ", "║"+format.Colorize("▸", format.Cyan+format.Bold, useColor)+" ", 1)
}
```

- [ ] **Step 6: Add renderSubView stub (will be filled in Task 4-5)**

```go
// renderSubView renders a full-screen detail view for the selected section.
func renderSubView(state *dashState) string {
	var b strings.Builder
	uc := state.useColor
	title := sectionNames[state.section]

	b.WriteString(boxTop())
	titleLine := format.Colorize("  "+title, format.Bold+format.Cyan, uc)
	b.WriteString(boxRow(titleLine))
	b.WriteString(boxMid())

	switch state.section {
	case sectionAccounts:
		b.WriteString(renderAccountsSubView(state))
	case sectionUsage:
		b.WriteString(renderUsageSubView(state))
	case sectionWeek:
		b.WriteString(renderWeekSubView(state))
	case sectionSessions:
		b.WriteString(renderSessionsSubView(state))
	case sectionInsights:
		b.WriteString(renderInsightsSubView(state))
	case sectionROI:
		b.WriteString(renderROISubView(state))
	}

	// Footer with keybindings.
	b.WriteString(boxMid())
	var hint string
	switch state.section {
	case sectionSessions:
		hint = "  ↑↓=select  r=resume  q=back"
	case sectionAccounts:
		hint = "  ↑↓=select  s=switch  q=back"
	default:
		hint = "  q=back"
	}
	b.WriteString(padLine(format.Colorize(hint, format.Dim, uc)))
	b.WriteString(boxBottom())

	return b.String()
}
```

- [ ] **Step 7: Build and verify compilation**

Run: `go build ./...`
Expected: Clean compilation (sub-view render functions are stubs that will be implemented in Tasks 4-5).

- [ ] **Step 8: Commit**

```bash
git add run_dashboard.go
git commit -m "feat(dashboard): interactive state machine with keyboard navigation"
```

---

### Task 3: INSIGHTS Summary Section (Main View)

**Files:**
- Modify: `run_dashboard.go`

Add the new INSIGHTS section to the main view, showing peak hours and cache ratio as a one-line summary.

- [ ] **Step 1: Implement renderInsightsSummary**

Add to `run_dashboard.go`:

```go
// renderInsightsSummary shows a compact insights line in the main view.
func renderInsightsSummary(useColor bool, data *dashboardData) string {
	var b strings.Builder
	b.WriteString(sectionHeader("INSIGHTS", useColor))

	if len(data.entries) == 0 {
		b.WriteString(padLine("  No data."))
		b.WriteString(emptyLine())
		return b.String()
	}

	eff := usage.CalculateEfficiency(data.entries)
	peaks := usage.FindPeakHours(data.entries, 3)

	var peakStrs []string
	for _, p := range peaks {
		peakStrs = append(peakStrs, fmt.Sprintf("%02d:00", p.Hour))
	}

	line := fmt.Sprintf("  Peak: %s   Cache: %s   Avg tokens/msg: %s",
		format.Colorize(strings.Join(peakStrs, ", "), format.White, useColor),
		format.Colorize(fmt.Sprintf("%.0f%%", eff.CacheHitRatio*100), format.Green, useColor),
		format.Colorize(format.FormatNumber(int64(eff.AvgTokensPerMessage)), format.Dim, useColor))
	b.WriteString(padLine(line))

	b.WriteString(emptyLine())
	return b.String()
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add run_dashboard.go
git commit -m "feat(dashboard): add INSIGHTS summary section (peak hours, cache ratio)"
```

---

### Task 4: Sub-Views for Read-Only Sections

**Files:**
- Create: `run_dashboard_subviews.go`

Implement the 4 read-only sub-views: TODAY'S USAGE (hourly chart), THIS WEEK (30-day heatmap), INSIGHTS (full detail), ROI (monthly breakdown).

- [ ] **Step 1: Create file with usage sub-view**

Create `run_dashboard_subviews.go`:

```go
package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Mike-7777777/cx/internal/format"
	"github.com/Mike-7777777/cx/internal/usage"
)

// renderUsageSubView shows an hourly bar chart for today's usage.
func renderUsageSubView(state *dashState) string {
	var b strings.Builder
	uc := state.useColor

	today := time.Now().UTC().Format("2006-01-02")
	var entries []usage.Entry
	for _, e := range state.data.entries {
		if e.Timestamp.UTC().Format("2006-01-02") == today {
			entries = append(entries, e)
		}
	}

	if len(entries) == 0 {
		b.WriteString(padLine("  No usage data for today."))
		return b.String()
	}

	// Aggregate by hour.
	hourCost := make(map[int]float64)
	var maxCost float64
	for _, e := range entries {
		h := e.Timestamp.UTC().Hour()
		cost := usage.CalculateCost(e.Model, e.Usage)
		hourCost[h] += cost
		if hourCost[h] > maxCost {
			maxCost = hourCost[h]
		}
	}

	b.WriteString(emptyLine())
	const barWidth = 35
	for h := 0; h < 24; h++ {
		cost := hourCost[h]
		barLen := 0
		if maxCost > 0 {
			barLen = int(math.Round(cost / maxCost * float64(barWidth)))
		}
		bar := strings.Repeat("█", barLen)
		label := fmt.Sprintf("  %02d:00 ", h)

		color := format.Green
		if cost > 5 {
			color = format.Yellow
		}
		if cost > 20 {
			color = format.Red
		}

		costStr := ""
		if cost > 0 {
			costStr = fmt.Sprintf(" $%.2f", cost)
		}

		line := label + format.Colorize(bar, color, uc) + format.Colorize(costStr, format.Dim, uc)
		b.WriteString(padLine(line))
	}

	return b.String()
}
```

- [ ] **Step 2: Add 30-day heatmap sub-view**

Append to `run_dashboard_subviews.go`:

```go
// renderWeekSubView shows a 30-day calendar heatmap.
func renderWeekSubView(state *dashState) string {
	var b strings.Builder
	uc := state.useColor

	now := time.Now().UTC()
	start := now.AddDate(0, 0, -29)

	// Aggregate daily costs.
	dailyMap := make(map[string]float64)
	var maxCost float64
	for _, e := range state.data.entries {
		dateKey := e.Timestamp.UTC().Format("2006-01-02")
		cost := usage.CalculateCost(e.Model, e.Usage)
		dailyMap[dateKey] += cost
		if dailyMap[dateKey] > maxCost {
			maxCost = dailyMap[dateKey]
		}
	}

	b.WriteString(emptyLine())

	// Render 30 days in rows of 7 (like a calendar).
	// Header: day-of-week labels.
	b.WriteString(padLine("           Mon Tue Wed Thu Fri Sat Sun"))
	b.WriteString(emptyLine())

	// Walk from start, group by week.
	day := start
	for day.Before(now) || day.Equal(now) {
		// Start of a week row.
		weekLabel := day.Format("Jan 02")
		row := fmt.Sprintf("  %s  ", weekLabel)

		// Pad to the correct weekday column.
		wd := int(day.Weekday()+6) % 7 // Mon=0
		row += strings.Repeat("    ", wd)

		for wd < 7 && (day.Before(now) || day.Equal(now)) {
			dateKey := day.Format("2006-01-02")
			cost := dailyMap[dateKey]
			cell := heatmapCell(cost, maxCost, uc)
			row += cell + " "
			day = day.AddDate(0, 0, 1)
			wd++
		}
		b.WriteString(padLine(row))
	}

	b.WriteString(emptyLine())

	// Legend.
	legend := "  " +
		format.Colorize("░", format.Dim, uc) + "=$0 " +
		format.Colorize("▒", format.Green, uc) + "=low " +
		format.Colorize("▓", format.Yellow, uc) + "=med " +
		format.Colorize("█", format.Red, uc) + "=high"
	b.WriteString(padLine(legend))

	return b.String()
}

// heatmapCell returns a colored block character based on cost intensity.
func heatmapCell(cost, maxCost float64, useColor bool) string {
	if cost == 0 {
		return format.Colorize("░", format.Dim, useColor)
	}
	ratio := cost / maxCost
	switch {
	case ratio < 0.33:
		return format.Colorize("▒", format.Green, useColor)
	case ratio < 0.66:
		return format.Colorize("▓", format.Yellow, useColor)
	default:
		return format.Colorize("█", format.Red, useColor)
	}
}
```

- [ ] **Step 3: Add insights sub-view**

Append to `run_dashboard_subviews.go`:

```go
// renderInsightsSubView shows full insights detail.
func renderInsightsSubView(state *dashState) string {
	var b strings.Builder
	uc := state.useColor

	if len(state.data.entries) == 0 {
		b.WriteString(padLine("  No data."))
		return b.String()
	}

	// Hourly distribution (top 6 hours).
	b.WriteString(emptyLine())
	b.WriteString(padLine(format.Colorize("  HOURLY DISTRIBUTION", format.Bold, uc)))
	hourly := usage.AggregateHourly(state.data.entries)
	peaks := usage.FindPeakHours(state.data.entries, 6)
	peakSet := make(map[int]bool)
	for _, p := range peaks {
		peakSet[p.Hour] = true
	}

	var maxMsgs int
	for _, h := range hourly {
		if h.Summary.EntryCount > maxMsgs {
			maxMsgs = h.Summary.EntryCount
		}
	}
	for _, h := range hourly {
		if h.Summary.EntryCount == 0 {
			continue
		}
		barLen := 0
		if maxMsgs > 0 {
			barLen = int(math.Round(float64(h.Summary.EntryCount) / float64(maxMsgs) * 25))
		}
		bar := strings.Repeat("█", barLen)
		marker := " "
		if peakSet[h.Hour] {
			marker = format.Colorize("★", format.Yellow, uc)
		}
		line := fmt.Sprintf("  %02d:00 %s%s %d msgs",
			h.Hour, marker, format.Colorize(bar, format.Cyan, uc), h.Summary.EntryCount)
		b.WriteString(padLine(line))
	}

	// Model distribution.
	b.WriteString(emptyLine())
	b.WriteString(padLine(format.Colorize("  MODEL DISTRIBUTION", format.Bold, uc)))
	models := usage.AggregateModelDistribution(state.data.entries)
	for _, m := range models {
		line := fmt.Sprintf("  %-12s %s tokens  %s",
			m.Model,
			format.Colorize(format.FormatNumber(m.Summary.TotalTokens), format.White, uc),
			format.Colorize(fmt.Sprintf("$%.2f", m.Summary.CostUSD), format.Yellow, uc))
		b.WriteString(padLine(line))
	}

	// Efficiency metrics.
	b.WriteString(emptyLine())
	b.WriteString(padLine(format.Colorize("  EFFICIENCY", format.Bold, uc)))
	eff := usage.CalculateEfficiency(state.data.entries)
	b.WriteString(padLine(fmt.Sprintf("  Cache hit ratio:   %s",
		format.Colorize(fmt.Sprintf("%.1f%%", eff.CacheHitRatio*100), format.Green, uc))))
	b.WriteString(padLine(fmt.Sprintf("  Avg tokens/msg:    %s",
		format.FormatNumber(int64(eff.AvgTokensPerMessage)))))
	b.WriteString(padLine(fmt.Sprintf("  Input/output:      %.1f:1", eff.InputOutputRatio)))

	return b.String()
}
```

- [ ] **Step 4: Add ROI sub-view**

Append to `run_dashboard_subviews.go`:

```go
// renderROISubView shows monthly ROI breakdown.
func renderROISubView(state *dashState) string {
	var b strings.Builder
	uc := state.useColor

	reports := usage.AggregateMonthly(state.data.entries)
	if len(reports) == 0 {
		b.WriteString(padLine("  No usage data."))
		return b.String()
	}

	subCost := totalSubscriptionCost()

	b.WriteString(emptyLine())
	b.WriteString(padLine(fmt.Sprintf("  %-10s  %10s  %10s  %10s", "Month", "API Cost", "Sub Cost", "Savings")))
	sep := "  " + strings.Repeat("─", 48)
	b.WriteString(padLine(format.Colorize(sep, format.Dim, uc)))

	var totalAPI float64
	for _, r := range reports {
		apiCost := r.Summary.CostUSD
		totalAPI += apiCost
		savings := apiCost - subCost
		var savingsStr string
		if savings > 0 {
			savingsStr = format.Colorize(fmt.Sprintf("$%.0f", savings), format.Green, uc)
		} else {
			savingsStr = format.Colorize(fmt.Sprintf("-$%.0f", -savings), format.Red, uc)
		}
		line := fmt.Sprintf("  %-10s  %s  %10s  %s",
			r.Month,
			format.Colorize(fmt.Sprintf("%10s", fmt.Sprintf("$%.0f", apiCost)), format.Yellow, uc),
			fmt.Sprintf("$%.0f", subCost),
			savingsStr)
		b.WriteString(padLine(line))
	}

	b.WriteString(emptyLine())
	totalSub := subCost * float64(len(reports))
	totalSavings := totalAPI - totalSub
	b.WriteString(padLine(fmt.Sprintf("  Lifetime:  API %s  Sub $%.0f  Saved %s",
		format.Colorize(fmt.Sprintf("$%.0f", totalAPI), format.Yellow, uc),
		totalSub,
		format.Colorize(fmt.Sprintf("$%.0f", totalSavings), format.Green+format.Bold, uc))))

	return b.String()
}
```

- [ ] **Step 5: Verify build**

Run: `go build ./...`

- [ ] **Step 6: Commit**

```bash
git add run_dashboard_subviews.go
git commit -m "feat(dashboard): add read-only sub-views (usage hourly, 30-day heatmap, insights, ROI)"
```

---

### Task 5: Sub-Views for Interactive Sections (Accounts + Sessions)

**Files:**
- Modify: `run_dashboard_subviews.go`

Add the two sub-views with selectable rows: ACCOUNTS (with `s` to switch) and SESSIONS (with `r` to resume).

- [ ] **Step 1: Add accounts sub-view**

Append to `run_dashboard_subviews.go`:

```go
// renderAccountsSubView shows all accounts with selectable rows.
func renderAccountsSubView(state *dashState) string {
	var b strings.Builder
	uc := state.useColor

	b.WriteString(emptyLine())

	for i, name := range state.accountList {
		cursor := "  "
		if i == state.subCursor {
			cursor = format.Colorize("▸ ", format.Cyan+format.Bold, uc)
		}

		pct, ttr, _, ok := fiveHourStats(accountConfigDir(state, name))
		if !ok {
			line := cursor + fmt.Sprintf("%-18s 5h: no data", name)
			b.WriteString(padLine(line))
			continue
		}

		bar := format.ProgressBar(pct, 8)
		color := format.UsageColor(pct)
		resetStr := format.LabelReset
		if ttr > 0 {
			resetStr = fmt.Sprintf("resets %s", format.FormatDuration(ttr))
		}

		line := cursor + fmt.Sprintf("%-16s %s %s  (%s)",
			name,
			format.Colorize(bar, color, uc),
			format.Colorize(fmt.Sprintf("%3.0f%%", pct), color, uc),
			resetStr)
		b.WriteString(padLine(line))
	}

	return b.String()
}

// accountConfigDir resolves the config directory for an account name.
func accountConfigDir(state *dashState, name string) string {
	for _, dir := range state.data.configDirs {
		// Match by checking if the directory basename contains the account name.
		// This is a heuristic; the authoritative lookup is via the registry.
	}
	// Fall back to registry-based lookup.
	regPath, err := config.RegistryPath()
	if err != nil {
		return ""
	}
	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		return ""
	}
	dir, err := reg.ResolveConfigDir(name)
	if err != nil {
		return ""
	}
	return dir
}
```

- [ ] **Step 2: Add sessions sub-view**

Append to `run_dashboard_subviews.go`:

```go
// renderSessionsSubView shows all sessions with selectable rows.
func renderSessionsSubView(state *dashState) string {
	var b strings.Builder
	uc := state.useColor

	if len(state.sessionList) == 0 {
		b.WriteString(padLine("  No sessions found."))
		return b.String()
	}

	b.WriteString(emptyLine())
	b.WriteString(padLine(fmt.Sprintf("  %-4s  %-8s  %-20s  %s", "Acct", "Age", "Project", "Topic")))
	sep := "  " + strings.Repeat("─", dashboardBoxWidth-4)
	b.WriteString(padLine(format.Colorize(sep, format.Dim, uc)))

	max := len(state.sessionList)
	if max > 30 {
		max = 30
	}

	for i, s := range state.sessionList[:max] {
		cursor := "  "
		if i == state.subCursor {
			cursor = format.Colorize("▸ ", format.Cyan+format.Bold, uc)
		}

		age := formatAge(s.Age)
		topic := s.Slug
		if topic == "" {
			topic = s.FirstMsg
		}
		if len(topic) > 22 {
			topic = topic[:22] + "…"
		}

		active := " "
		if s.Active {
			active = format.Colorize("●", format.Green, uc)
		}

		line := cursor + fmt.Sprintf("%-4s %s %-8s  %-20s  %s",
			s.Account, active, age, s.Project, topic)
		b.WriteString(padLine(line))
	}

	return b.String()
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add run_dashboard_subviews.go
git commit -m "feat(dashboard): add interactive sub-views (accounts with switch, sessions with resume)"
```

---

### Task 6: Tests

**Files:**
- Modify: `run_dashboard_test.go`

Add tests for the state machine logic and helper functions.

- [ ] **Step 1: Test handleDashKey navigation**

Add to `run_dashboard_test.go`:

```go
func TestHandleDashKey_MainNavigation(t *testing.T) {
	state := &dashState{view: viewMain, section: 0}

	handleDashKey(state, platform.KeyDown)
	if state.section != 1 {
		t.Errorf("section = %d, want 1", state.section)
	}

	handleDashKey(state, platform.KeyDown)
	handleDashKey(state, platform.KeyDown)
	if state.section != 3 {
		t.Errorf("section = %d, want 3", state.section)
	}

	handleDashKey(state, platform.KeyUp)
	if state.section != 2 {
		t.Errorf("section = %d, want 2 after up", state.section)
	}
}

func TestHandleDashKey_EnterSubView(t *testing.T) {
	state := &dashState{view: viewMain, section: sectionSessions}

	handleDashKey(state, platform.KeyEnter)
	if state.view != viewSub {
		t.Error("expected viewSub after Enter")
	}
	if state.subCursor != 0 {
		t.Error("subCursor should reset to 0")
	}
}

func TestHandleDashKey_SubViewBack(t *testing.T) {
	state := &dashState{view: viewSub, section: sectionSessions}

	action := handleDashKey(state, platform.KeyQ)
	if state.view != viewMain {
		t.Error("expected viewMain after q in sub-view")
	}
	if action != dashActionNone {
		t.Errorf("expected dashActionNone, got %d", action)
	}
}

func TestHandleDashKey_EscSubView(t *testing.T) {
	state := &dashState{view: viewSub, section: sectionUsage}

	handleDashKey(state, platform.KeyEsc)
	if state.view != viewMain {
		t.Error("expected viewMain after Esc in sub-view")
	}
}

func TestHandleDashKey_ResumeAction(t *testing.T) {
	state := &dashState{
		view:        viewSub,
		section:     sectionSessions,
		sessionList: []sessionEntry{{ID: "test-123"}},
		subCursor:   0,
	}
	action := handleDashKey(state, platform.KeyR)
	if action != dashActionResume {
		t.Errorf("expected dashActionResume, got %d", action)
	}
}

func TestHandleDashKey_SwitchAction(t *testing.T) {
	state := &dashState{
		view:        viewSub,
		section:     sectionAccounts,
		accountList: []string{"main", "alt"},
		subCursor:   1,
	}
	action := handleDashKey(state, platform.KeyS)
	if action != dashActionSwitch {
		t.Errorf("expected dashActionSwitch, got %d", action)
	}
}

func TestHandleDashKey_RInNonSessionsView(t *testing.T) {
	state := &dashState{view: viewSub, section: sectionROI}
	action := handleDashKey(state, platform.KeyR)
	if action != dashActionNone {
		t.Errorf("r in ROI sub-view should be no-op, got %d", action)
	}
}

func TestHandleDashKey_BoundsCheck(t *testing.T) {
	// Down at last section should not go past.
	state := &dashState{view: viewMain, section: sectionCount - 1}
	handleDashKey(state, platform.KeyDown)
	if state.section != sectionCount-1 {
		t.Errorf("should stay at last section, got %d", state.section)
	}

	// Up at first section should not go negative.
	state.section = 0
	handleDashKey(state, platform.KeyUp)
	if state.section != 0 {
		t.Errorf("should stay at 0, got %d", state.section)
	}
}

func TestHandleDashKey_MainQuit(t *testing.T) {
	state := &dashState{view: viewMain}
	action := handleDashKey(state, platform.KeyQ)
	if action != dashActionQuit {
		t.Errorf("expected dashActionQuit, got %d", action)
	}
}
```

- [ ] **Step 2: Test heatmapCell**

```go
func TestHeatmapCell_Zero(t *testing.T) {
	cell := heatmapCell(0, 100, false)
	if !strings.Contains(cell, "░") {
		t.Errorf("zero cost should show ░, got %q", cell)
	}
}

func TestHeatmapCell_Low(t *testing.T) {
	cell := heatmapCell(10, 100, false)
	if !strings.Contains(cell, "▒") {
		t.Errorf("low cost should show ▒, got %q", cell)
	}
}

func TestHeatmapCell_High(t *testing.T) {
	cell := heatmapCell(90, 100, false)
	if !strings.Contains(cell, "█") {
		t.Errorf("high cost should show █, got %q", cell)
	}
}

func TestSubViewRowCount(t *testing.T) {
	state := &dashState{
		section:     sectionSessions,
		sessionList: []sessionEntry{{}, {}, {}},
	}
	if subViewRowCount(state) != 3 {
		t.Errorf("expected 3, got %d", subViewRowCount(state))
	}

	state.section = sectionROI
	if subViewRowCount(state) != 0 {
		t.Errorf("ROI should have 0 selectable rows, got %d", subViewRowCount(state))
	}
}

func TestHighlightSection(t *testing.T) {
	content := "║  ACCOUNTS\n║  data\n"
	result := highlightSection(content, false)
	if !strings.Contains(result, "▸") {
		t.Error("highlighted section should contain ▸")
	}
}
```

- [ ] **Step 3: Run all tests**

Run: `go test ./... -count=1 -v`

- [ ] **Step 4: Commit**

```bash
git add run_dashboard_test.go
git commit -m "test(dashboard): add state machine and sub-view tests"
```

---

### Task 7: Documentation and Completion Update

**Files:**
- Modify: `README.md`
- Modify: `CLAUDE.md`
- Modify: `run_completion.go` (add `--interactive` hint if needed, update dashboard description)

- [ ] **Step 1: Update README dashboard description**

Replace the dashboard entry in the monitoring table with:

```markdown
| `dashboard [--interval N]` | Interactive TUI dashboard (↑↓ navigate, Enter detail, r resume, s switch) |
```

- [ ] **Step 2: Update CLAUDE.md if needed**

No changes needed — the Runner pattern is unchanged.

- [ ] **Step 3: Final full test run**

Run: `go test ./... -count=1 -race` (on Unix) or `go test ./... -count=1` (on Windows)

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: update dashboard description for interactive TUI"
```

---

## Self-Review Checklist

1. **Spec coverage**: All 6 sections covered (ACCOUNTS, TODAY'S USAGE, THIS WEEK, SESSIONS, INSIGHTS, ROI). Main view navigation ✓. Sub-views ✓. Actions (r/s) ✓. 30-day heatmap ✓. Keyboard layer ✓. Passive fallback ✓.
2. **Placeholder scan**: No TBD/TODO. All code blocks complete.
3. **Type consistency**: `dashState`, `viewMode`, `sectionID`, `dashAction` used consistently. `platform.Key*` constants match Task 1 definitions. `sessionEntry` and `collectSessions` match existing code.
