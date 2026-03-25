package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Mike-7777777/cx/internal/cache"
	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/format"
	"github.com/Mike-7777777/cx/internal/platform"
	"github.com/Mike-7777777/cx/internal/usage"
)

const (
	defaultDashboardInterval = 5 * time.Second
	dashboardBoxWidth        = 64
)

// sessionMetadata mirrors the JSON structure in ~/.claude/sessions/<pid>.json.
type sessionMetadata struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
	StartedAt int64  `json:"startedAt"` // Unix milliseconds
}

// sessionInfo holds a parsed, filtered session for display.
type sessionInfo struct {
	PID       int
	Cwd       string
	StartedAt time.Time
}

func runDashboard() {
	interval := defaultDashboardInterval

	// Parse --interval flag.
	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		if args[i] == "--interval" {
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "cx dashboard: --interval requires a value in seconds")
				os.Exit(1)
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 {
				fmt.Fprintf(os.Stderr, "cx dashboard: --interval must be a positive integer, got %q\n", args[i])
				os.Exit(1)
			}
			interval = time.Duration(n) * time.Second
		} else {
			fmt.Fprintf(os.Stderr, "cx dashboard: unknown flag %q\n", args[i])
			os.Exit(1)
		}
	}

	// Handle Ctrl+C / SIGTERM for clean shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	useColor := platform.ANSIEnabled()

	// Initial render.
	renderDashboard(useColor, interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-quit:
			// Restore terminal: show cursor, reset colors.
			fmt.Print("\033[?25h" + format.Reset)
			return
		case <-ticker.C:
			renderDashboard(useColor, interval)
		}
	}
}

// renderDashboard clears the screen and draws the full dashboard frame.
func renderDashboard(useColor bool, interval time.Duration) {
	var b strings.Builder

	// Clear screen and move cursor to top-left. Hide cursor during redraw.
	b.WriteString("\033[2J\033[H\033[?25l")

	// Title bar.
	b.WriteString(boxTop())
	title := "  cx dashboard"
	refreshNote := fmt.Sprintf("[refreshing every %ds]", int(interval.Seconds()))
	padding := dashboardBoxWidth - len(title) - len(refreshNote) - 4
	if padding < 1 {
		padding = 1
	}
	titleLine := format.Colorize(title, format.Bold+format.Cyan, useColor) +
		strings.Repeat(" ", padding) +
		format.Colorize(refreshNote, format.Dim, useColor)
	b.WriteString(boxRow(titleLine))
	b.WriteString(boxMid())

	// Section: ACCOUNTS
	b.WriteString(renderAccountsSection(useColor))

	// Section: TODAY'S USAGE
	b.WriteString(renderTodayUsageSection(useColor))

	// Section: THIS WEEK
	b.WriteString(renderWeeklyChartSection(useColor))

	// Section: ACTIVE SESSIONS
	b.WriteString(renderSessionsSection(useColor))

	// Section: ROI
	b.WriteString(renderROISection(useColor))

	// Bottom border.
	b.WriteString(boxBottom())

	fmt.Print(b.String())
}

// ---------- ACCOUNTS SECTION ----------

func renderAccountsSection(useColor bool) string {
	var b strings.Builder

	b.WriteString(sectionHeader("ACCOUNTS", useColor))

	regPath, err := config.RegistryPath()
	if err != nil {
		b.WriteString(padLine("  (registry not found)"))
		return b.String()
	}

	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		b.WriteString(padLine("  (registry error)"))
		return b.String()
	}

	if len(reg.Accounts) == 0 {
		b.WriteString(padLine("  No accounts configured. Run: cx init"))
		return b.String()
	}

	names := make([]string, 0, len(reg.Accounts))
	for name := range reg.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)

	var bestName string
	var bestPct float64 = 999

	for _, name := range names {
		dir, err := reg.ResolveConfigDir(name)
		if err != nil {
			b.WriteString(padLine(fmt.Sprintf("  %-14s  no data", name)))
			continue
		}

		rc, err := cache.ReadRateCache(filepath.Join(dir, "rate-cache.json"))
		if err != nil || rc == nil || rc.RateLimits == nil || rc.RateLimits.FiveHour == nil {
			label := name
			if name == reg.Main {
				label += " (main)"
			}
			line := fmt.Sprintf("  %-18s 5h: no data", label)
			b.WriteString(padLine(line))
			continue
		}

		rl := rc.RateLimits
		label := name
		if name == reg.Main {
			label += " (main)"
		}

		// 5-hour info.
		var fivePct float64
		var fiveBar, fiveReset string
		if rl.FiveHour.IsReset() {
			fivePct = 0
			fiveBar = format.ProgressBar(0, 5)
			fiveReset = format.LabelReset
		} else {
			fivePct = rl.FiveHour.UsedPercentage
			fiveBar = format.ProgressBar(fivePct, 5)
			fiveReset = fmt.Sprintf("resets %s", format.FormatDuration(rl.FiveHour.TimeToReset()))
		}

		fiveColor := format.UsageColor(fivePct)
		fiveStr := format.Colorize(fiveBar, fiveColor, useColor) + " " +
			format.Colorize(fmt.Sprintf("%3.0f%%", fivePct), fiveColor, useColor)

		// 7-day info.
		sevenStr := ""
		if rl.SevenDay != nil {
			sevenPct := rl.SevenDay.UsedPercentage
			sevenColor := format.UsageColor(sevenPct)
			sevenStr = fmt.Sprintf("  7d: %s",
				format.Colorize(fmt.Sprintf("%.0f%%", sevenPct), sevenColor, useColor))
		}

		line := fmt.Sprintf("  %-18s 5h: %s  (%s)%s", label, fiveStr, fiveReset, sevenStr)
		b.WriteString(padLine(line))

		if fivePct < bestPct {
			bestPct = fivePct
			bestName = name
		}
	}

	if bestName != "" {
		rec := fmt.Sprintf("  %s Recommended: %s",
			format.Colorize("✓", format.Green, useColor),
			format.Colorize(bestName, format.Green+format.Bold, useColor))
		b.WriteString(padLine(rec))
	}

	b.WriteString(emptyLine())
	return b.String()
}

// ---------- TODAY'S USAGE SECTION ----------

func renderTodayUsageSection(useColor bool) string {
	var b strings.Builder

	b.WriteString(sectionHeader("TODAY'S USAGE", useColor))

	configDirs, err := allRegistryDirs()
	if err != nil {
		dir, err2 := config.DetectConfigDir()
		if err2 != nil {
			b.WriteString(padLine("  (no config dir found)"))
			return b.String()
		}
		configDirs = []string{dir}
	}

	// Scan today's entries.
	today := time.Now().UTC().Format("2006-01-02")
	var entries []usage.Entry
	for _, dir := range configDirs {
		_ = usage.ScanDir(dir, func(e usage.Entry) {
			if e.Timestamp.UTC().Format("2006-01-02") == today {
				entries = append(entries, e)
			}
		})
	}

	if len(entries) == 0 {
		b.WriteString(padLine("  No usage data for today."))
		b.WriteString(emptyLine())
		return b.String()
	}

	// Aggregate.
	var totalTokens int64
	var totalCost float64
	modelTokens := make(map[string]int64)
	for _, e := range entries {
		tokens := e.Usage.InputTokens + e.Usage.OutputTokens +
			e.Usage.CacheCreationInputTokens + e.Usage.CacheReadInputTokens
		totalTokens += tokens
		totalCost += usage.CalculateCost(e.Model, e.Usage)
		modelTokens[shortModelName(e.Model)] += tokens
	}

	line1 := fmt.Sprintf("  Tokens: %s    Cost: %s    Messages: %s",
		format.Colorize(format.FormatNumber(totalTokens), format.White, useColor),
		format.Colorize(fmt.Sprintf("$%.2f", totalCost), format.Yellow, useColor),
		format.Colorize(fmt.Sprintf("%d", len(entries)), format.White, useColor))
	b.WriteString(padLine(line1))

	// Model breakdown by percentage.
	type modelPct struct {
		name string
		pct  float64
	}
	var modelPcts []modelPct
	for name, tokens := range modelTokens {
		pct := float64(tokens) / float64(totalTokens) * 100
		modelPcts = append(modelPcts, modelPct{name, pct})
	}
	sort.Slice(modelPcts, func(i, j int) bool { return modelPcts[i].pct > modelPcts[j].pct })

	var parts []string
	for _, mp := range modelPcts {
		parts = append(parts, fmt.Sprintf("%s %.0f%%", mp.name, mp.pct))
	}
	line2 := "  Models: " + strings.Join(parts, ", ")
	b.WriteString(padLine(line2))

	b.WriteString(emptyLine())
	return b.String()
}

// ---------- WEEKLY CHART SECTION ----------

func renderWeeklyChartSection(useColor bool) string {
	var b strings.Builder

	b.WriteString(sectionHeader("THIS WEEK", useColor))

	configDirs, err := allRegistryDirs()
	if err != nil {
		dir, err2 := config.DetectConfigDir()
		if err2 != nil {
			b.WriteString(padLine("  (no data)"))
			return b.String()
		}
		configDirs = []string{dir}
	}

	// Determine the date range: last 7 days.
	now := time.Now().UTC()
	todayStr := now.Format("2006-01-02")
	weekStart := now.AddDate(0, 0, -6)
	weekStartStr := weekStart.Format("2006-01-02")

	var entries []usage.Entry
	for _, dir := range configDirs {
		_ = usage.ScanDir(dir, func(e usage.Entry) {
			dateKey := e.Timestamp.UTC().Format("2006-01-02")
			if dateKey >= weekStartStr && dateKey <= todayStr {
				entries = append(entries, e)
			}
		})
	}

	if len(entries) == 0 {
		b.WriteString(padLine("  No usage data this week."))
		b.WriteString(emptyLine())
		return b.String()
	}

	dailies := usage.AggregateDailies(entries)

	// Build map for quick lookup.
	dailyMap := make(map[string]float64)
	for _, d := range dailies {
		dailyMap[d.Date] = d.Summary.CostUSD
	}

	// Find max cost for scaling.
	var maxCost float64
	for _, cost := range dailyMap {
		if cost > maxCost {
			maxCost = cost
		}
	}

	// Render bars for each day.
	const maxBarWidth = 30
	weekdays := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

	for i := 0; i < 7; i++ {
		day := weekStart.AddDate(0, 0, i)
		dateKey := day.Format("2006-01-02")
		cost := dailyMap[dateKey]
		wd := weekdays[int(day.Weekday()+6)%7] // Convert Go's Sunday=0 to Monday-first

		barLen := 0
		if maxCost > 0 {
			barLen = int(math.Round(cost / maxCost * float64(maxBarWidth)))
		}

		bar := strings.Repeat("█", barLen)
		costStr := fmt.Sprintf("$%.0f", cost)

		todayMarker := ""
		if dateKey == todayStr {
			todayMarker = format.Colorize("  ← today", format.Cyan, useColor)
		}

		barColor := format.Green
		if cost > 200 {
			barColor = format.Yellow
		}
		if cost > 500 {
			barColor = format.Red
		}

		line := fmt.Sprintf("  %s   %s  %s%s",
			wd,
			format.Colorize(bar, barColor, useColor),
			format.Colorize(costStr, format.Dim, useColor),
			todayMarker)
		b.WriteString(padLine(line))
	}

	b.WriteString(emptyLine())
	return b.String()
}

// ---------- SESSIONS SECTION ----------

func renderSessionsSection(useColor bool) string {
	var b strings.Builder

	b.WriteString(sectionHeader("ACTIVE SESSIONS", useColor))

	sessions := getActiveSessions()
	if len(sessions) == 0 {
		b.WriteString(padLine("  No active sessions."))
		b.WriteString(emptyLine())
		return b.String()
	}

	for _, s := range sessions {
		dur := time.Since(s.StartedAt)
		durStr := format.FormatDuration(dur)
		cwd := shortenPath(s.Cwd)
		line := fmt.Sprintf("  PID %-8d %-24s started %s ago",
			s.PID, cwd, durStr)
		b.WriteString(padLine(line))
	}

	b.WriteString(emptyLine())
	return b.String()
}

func getActiveSessions() []sessionInfo {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	sessDir := filepath.Join(home, ".claude", "sessions")
	entries, err := os.ReadDir(sessDir)
	if err != nil {
		return nil
	}

	var sessions []sessionInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(sessDir, entry.Name()))
		if err != nil {
			continue
		}

		var meta sessionMetadata
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}

		// Check if PID is still running.
		if !isProcessRunning(meta.PID) {
			continue
		}

		startedAt := time.UnixMilli(meta.StartedAt)
		sessions = append(sessions, sessionInfo{
			PID:       meta.PID,
			Cwd:       meta.Cwd,
			StartedAt: startedAt,
		})
	}

	// Sort by start time (oldest first).
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartedAt.Before(sessions[j].StartedAt)
	})

	return sessions
}

// isProcessRunning checks if a process with the given PID exists.
// On Unix, signal 0 succeeds for any process owned by the current user,
// so this may return true for unrelated processes that reused the PID.
// This is acceptable for dashboard display purposes (stale sessions are
// cleaned up on the next refresh).
func isProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. Send signal 0 to check.
	// On Windows, FindProcess fails if the process doesn't exist.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// ---------- ROI SECTION ----------

func renderROISection(useColor bool) string {
	var b strings.Builder

	// Calculate monthly total from current month's data.
	configDirs, err := allRegistryDirs()
	if err != nil {
		dir, err2 := config.DetectConfigDir()
		if err2 != nil {
			return ""
		}
		configDirs = []string{dir}
	}

	now := time.Now().UTC()
	monthPrefix := now.Format("2006-01")
	var monthCost float64
	for _, dir := range configDirs {
		_ = usage.ScanDir(dir, func(e usage.Entry) {
			if strings.HasPrefix(e.Timestamp.UTC().Format("2006-01-02"), monthPrefix) {
				monthCost += usage.CalculateCost(e.Model, e.Usage)
			}
		})
	}

	if monthCost == 0 {
		return ""
	}

	subCost := totalSubscriptionCost()
	var savingsPct float64
	if monthCost > 0 {
		savingsPct = (monthCost - subCost) / monthCost * 100
	}

	line := fmt.Sprintf("  ROI: %s/mo subscription → %s equiv API (%s saved)",
		format.Colorize(fmt.Sprintf("$%.0f", subCost), format.Dim, useColor),
		format.Colorize(fmt.Sprintf("$%s", format.FormatNumber(int64(monthCost))), format.Yellow, useColor),
		format.Colorize(fmt.Sprintf("%.1f%%", savingsPct), format.Green+format.Bold, useColor))

	b.WriteString(padLine(line))
	return b.String()
}

// ---------- BOX DRAWING HELPERS ----------

func boxTop() string {
	return "╔" + strings.Repeat("═", dashboardBoxWidth) + "╗\n"
}

func boxMid() string {
	return "╠" + strings.Repeat("═", dashboardBoxWidth) + "╣\n"
}

func boxBottom() string {
	return "╚" + strings.Repeat("═", dashboardBoxWidth) + "╝\n"
}

func boxRow(content string) string {
	// Box rows with side borders. Content is pre-formatted with ANSI codes,
	// so we cannot reliably pad to a fixed width. Use a simple approach.
	return "║ " + content + " ║\n"
}

func emptyLine() string {
	return "║" + strings.Repeat(" ", dashboardBoxWidth) + "║\n"
}

// padLine wraps content in box borders. Since content may contain ANSI escape
// codes, we don't try to pad to exact width (would require stripping escapes).
func padLine(content string) string {
	return "║" + content + "\n"
}

func sectionHeader(title string, useColor bool) string {
	var b strings.Builder
	b.WriteString(emptyLine())
	b.WriteString(padLine("  " + format.Colorize(title, format.Bold+format.White, useColor)))
	sep := "  " + strings.Repeat("─", dashboardBoxWidth-4)
	b.WriteString(padLine(format.Colorize(sep, format.Dim, useColor)))
	return b.String()
}

// ---------- UTILITY HELPERS ----------

// shortModelName extracts a short display name from a full model identifier.
// e.g. "claude-sonnet-4-20250514" → "sonnet"
func shortModelName(model string) string {
	lower := strings.ToLower(model)
	switch {
	case strings.Contains(lower, "opus"):
		return "opus"
	case strings.Contains(lower, "sonnet"):
		return "sonnet"
	case strings.Contains(lower, "haiku"):
		return "haiku"
	default:
		// Return the last segment after the last dash, or the whole name.
		parts := strings.Split(model, "-")
		if len(parts) > 1 {
			return parts[len(parts)-2]
		}
		return model
	}
}

// shortenPath shortens a file path for display.
// Replaces home dir prefix with ~ and takes the last directory component.
func shortenPath(p string) string {
	home, err := os.UserHomeDir()
	if err == nil {
		p = strings.Replace(p, home, "~", 1)
	}
	// Convert backslashes to forward slashes for display.
	p = strings.ReplaceAll(p, "\\", "/")

	// Return just the last path component if long.
	parts := strings.Split(p, "/")
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		if last != "" {
			return last
		}
		// If trailing slash, use second-to-last.
		if len(parts) > 1 {
			return parts[len(parts)-2]
		}
	}
	return p
}
