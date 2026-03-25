package usage

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Mike-7777777/cc-monitor/internal/format"
)

// formatNumber formats an int64 with comma separators (e.g., 1234567 → "1,234,567").
func formatNumber(n int64) string {
	if n < 0 {
		return "-" + formatNumber(-n)
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var b strings.Builder
	remainder := len(s) % 3
	if remainder > 0 {
		b.WriteString(s[:remainder])
	}
	for i := remainder; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// formatCost formats a USD cost as "$X.XX".
func formatCost(cost float64) string {
	return fmt.Sprintf("$%.2f", cost)
}

// formatDuration formats a duration as "Xh Ym" or "< 1m".
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "< 1m"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

// truncateSessionID returns at most 24 characters of a session ID.
func truncateSessionID(id string) string {
	if len(id) <= 24 {
		return id
	}
	return id[:21] + "..."
}

const separator = "━"

// repeatSep returns a separator line of the given width.
func repeatSep(width int, useColor bool) string {
	var b strings.Builder
	for i := 0; i < width; i++ {
		b.WriteString(separator)
	}
	line := b.String()
	return format.Colorize(line, format.Dim, useColor)
}

// costColor returns Green for low cost and Yellow for high cost (>$100/day).
func costColor(cost float64) string {
	if cost > 100 {
		return format.Yellow
	}
	return format.Green
}

// FormatDailyTable formats daily reports as a human-readable table.
func FormatDailyTable(reports []DailyReport, useColor bool) string {
	const (
		fmtRow = "%-13s %11s %11s %11s %13s %11s %9s\n"
		width  = 82
	)

	var b strings.Builder

	header := fmt.Sprintf(fmtRow, "Date", "Input", "Output", "Cache Read", "Cache Create", "Total", "Cost")
	b.WriteString(format.Colorize(header, format.Bold, useColor))
	b.WriteString(repeatSep(width, useColor))
	b.WriteString("\n")

	var total UsageSummary
	for _, r := range reports {
		dateStr := format.Colorize(r.Date, format.Cyan, useColor)
		costStr := format.Colorize(formatCost(r.Summary.CostUSD), costColor(r.Summary.CostUSD), useColor)
		b.WriteString(fmt.Sprintf(fmtRow,
			dateStr,
			formatNumber(r.Summary.InputTokens),
			formatNumber(r.Summary.OutputTokens),
			formatNumber(r.Summary.CacheReadInputTokens),
			formatNumber(r.Summary.CacheCreationInputTokens),
			formatNumber(r.Summary.TotalTokens),
			costStr,
		))
		total.InputTokens += r.Summary.InputTokens
		total.OutputTokens += r.Summary.OutputTokens
		total.CacheReadInputTokens += r.Summary.CacheReadInputTokens
		total.CacheCreationInputTokens += r.Summary.CacheCreationInputTokens
		total.TotalTokens += r.Summary.TotalTokens
		total.CostUSD += r.Summary.CostUSD
	}

	b.WriteString(repeatSep(width, useColor))
	b.WriteString("\n")

	totalRow := fmt.Sprintf(fmtRow,
		"Total",
		formatNumber(total.InputTokens),
		formatNumber(total.OutputTokens),
		formatNumber(total.CacheReadInputTokens),
		formatNumber(total.CacheCreationInputTokens),
		formatNumber(total.TotalTokens),
		formatCost(total.CostUSD),
	)
	b.WriteString(format.Colorize(totalRow, format.Bold, useColor))

	return b.String()
}

// FormatSessionTable formats session reports as a table.
func FormatSessionTable(reports []SessionReport, useColor bool) string {
	const (
		fmtRow = "%-25s %-21s %10s %11s %9s\n"
		width  = 80
	)

	var b strings.Builder

	header := fmt.Sprintf(fmtRow, "Session", "Start", "Duration", "Tokens", "Cost")
	b.WriteString(format.Colorize(header, format.Bold, useColor))
	b.WriteString(repeatSep(width, useColor))
	b.WriteString("\n")

	var totalTokens int64
	var totalCost float64
	for _, r := range reports {
		dur := r.EndTime.Sub(r.StartTime)
		sessionStr := format.Colorize(truncateSessionID(r.SessionID), format.Cyan, useColor)
		costStr := format.Colorize(formatCost(r.Summary.CostUSD), costColor(r.Summary.CostUSD), useColor)
		b.WriteString(fmt.Sprintf(fmtRow,
			sessionStr,
			r.StartTime.UTC().Format("2006-01-02 15:04"),
			formatDuration(dur),
			formatNumber(r.Summary.TotalTokens),
			costStr,
		))
		totalTokens += r.Summary.TotalTokens
		totalCost += r.Summary.CostUSD
	}

	b.WriteString(repeatSep(width, useColor))
	b.WriteString("\n")

	totalRow := fmt.Sprintf(fmtRow,
		"Total",
		"",
		"",
		formatNumber(totalTokens),
		formatCost(totalCost),
	)
	b.WriteString(format.Colorize(totalRow, format.Bold, useColor))

	return b.String()
}

// FormatBlockTable formats block reports as a table.
func FormatBlockTable(reports []BlockReport, useColor bool) string {
	const (
		fmtRow = "%-25s %-22s %11s %9s\n"
		width  = 70
	)

	var b strings.Builder

	header := fmt.Sprintf(fmtRow, "Block Start", "Block End", "Tokens", "Cost")
	b.WriteString(format.Colorize(header, format.Bold, useColor))
	b.WriteString(repeatSep(width, useColor))
	b.WriteString("\n")

	var totalTokens int64
	var totalCost float64
	for _, r := range reports {
		startStr := format.Colorize(r.StartTime.UTC().Format("2006-01-02 15:04"), format.Cyan, useColor)
		costStr := format.Colorize(formatCost(r.Summary.CostUSD), costColor(r.Summary.CostUSD), useColor)
		b.WriteString(fmt.Sprintf(fmtRow,
			startStr,
			r.EndTime.UTC().Format("2006-01-02 15:04"),
			formatNumber(r.Summary.TotalTokens),
			costStr,
		))
		totalTokens += r.Summary.TotalTokens
		totalCost += r.Summary.CostUSD
	}

	b.WriteString(repeatSep(width, useColor))
	b.WriteString("\n")

	totalRow := fmt.Sprintf(fmtRow,
		"Total",
		"",
		formatNumber(totalTokens),
		formatCost(totalCost),
	)
	b.WriteString(format.Colorize(totalRow, format.Bold, useColor))

	return b.String()
}

// FormatJSON marshals any report slice to indented JSON.
func FormatJSON(v any) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("json marshal: %w", err)
	}
	return string(data), nil
}
