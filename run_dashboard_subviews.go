package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Mike-7777777/cx/internal/format"
	"github.com/Mike-7777777/cx/internal/usage"
)

// ---------- USAGE SUB-VIEW ----------

// renderUsageSubView shows a 24-hour horizontal bar chart of today's costs,
// one row per hour (0-23).
func renderUsageSubView(state *dashState) string {
	if state.data == nil {
		return padLine("  No usage data.")
	}

	today := time.Now().UTC().Format("2006-01-02")
	var todayEntries []usage.Entry
	for _, e := range state.data.entries {
		if e.Timestamp.UTC().Format("2006-01-02") == today {
			todayEntries = append(todayEntries, e)
		}
	}

	if len(todayEntries) == 0 {
		return padLine("  No usage data for today.")
	}

	// Aggregate cost per hour.
	hourlyCost := make(map[int]float64)
	for _, e := range todayEntries {
		h := e.Timestamp.UTC().Hour()
		hourlyCost[h] += usage.CalculateCost(e.Model, e.Usage)
	}

	// Find max cost for bar scaling.
	var maxCost float64
	for _, cost := range hourlyCost {
		if cost > maxCost {
			maxCost = cost
		}
	}

	const maxBarWidth = 30
	var b strings.Builder

	for h := 0; h < 24; h++ {
		cost := hourlyCost[h]
		barLen := 0
		if maxCost > 0 {
			barLen = int(math.Round(cost / maxCost * float64(maxBarWidth)))
		}

		bar := strings.Repeat("█", barLen)
		costStr := fmt.Sprintf("$%.2f", cost)

		barColor := format.Green
		if cost > 5 {
			barColor = format.Yellow
		}
		if cost > 20 {
			barColor = format.Red
		}

		label := fmt.Sprintf("%02d:00", h)
		line := fmt.Sprintf("  %s  %s  %s",
			format.Colorize(label, format.Dim, state.useColor),
			format.Colorize(bar, barColor, state.useColor),
			format.Colorize(costStr, format.Dim, state.useColor))
		b.WriteString(padLine(line))
	}

	return b.String()
}

// ---------- WEEK SUB-VIEW ----------

// renderWeekSubView shows a 30-day calendar heatmap grouped into week rows
// (Mon-Sun) using block characters to indicate cost intensity.
func renderWeekSubView(state *dashState) string {
	if state.data == nil {
		return padLine("  No usage data.")
	}

	now := time.Now().UTC()
	startDate := now.AddDate(0, 0, -29) // 30 days including today

	// Aggregate cost per day.
	dailyCost := make(map[string]float64)
	for _, e := range state.data.entries {
		dateKey := e.Timestamp.UTC().Format("2006-01-02")
		dailyCost[dateKey] += usage.CalculateCost(e.Model, e.Usage)
	}

	// Find max cost for thresholding.
	var maxCost float64
	for d := startDate; !d.After(now); d = d.AddDate(0, 0, 1) {
		dateKey := d.Format("2006-01-02")
		if c := dailyCost[dateKey]; c > maxCost {
			maxCost = c
		}
	}

	// Build calendar grid: align to Monday.
	// Walk back from startDate to find the previous Monday.
	alignedStart := startDate
	for alignedStart.Weekday() != time.Monday {
		alignedStart = alignedStart.AddDate(0, 0, -1)
	}

	var b strings.Builder

	// Header row with day labels.
	header := "  Wk   Mon Tue Wed Thu Fri Sat Sun"
	b.WriteString(padLine(format.Colorize(header, format.Dim, state.useColor)))

	weekNum := 0
	for d := alignedStart; !d.After(now); {
		weekNum++
		var cells []string

		for wd := 0; wd < 7; wd++ {
			dateKey := d.Format("2006-01-02")
			if d.Before(startDate) || d.After(now) {
				cells = append(cells, " . ")
			} else {
				cost := dailyCost[dateKey]
				cell := heatmapCell(cost, maxCost, state.useColor)
				cells = append(cells, cell)
			}
			d = d.AddDate(0, 0, 1)
		}

		line := fmt.Sprintf("  %2d   %s", weekNum, strings.Join(cells, " "))
		b.WriteString(padLine(line))
	}

	// Legend row.
	b.WriteString(emptyLine())
	legend := fmt.Sprintf("  %s=none  %s=low  %s=med  %s=high",
		format.Colorize("░", format.Dim, state.useColor),
		format.Colorize("▒", format.Green, state.useColor),
		format.Colorize("▓", format.Yellow, state.useColor),
		format.Colorize("█", format.Red, state.useColor))
	b.WriteString(padLine(legend))

	return b.String()
}

// heatmapCell returns a colored block character based on cost relative to max.
func heatmapCell(cost, maxCost float64, useColor bool) string {
	if cost == 0 || maxCost == 0 {
		return format.Colorize(" ░ ", format.Dim, useColor)
	}
	ratio := cost / maxCost
	switch {
	case ratio > 0.66:
		return format.Colorize(" █ ", format.Red, useColor)
	case ratio > 0.33:
		return format.Colorize(" ▓ ", format.Yellow, useColor)
	default:
		return format.Colorize(" ▒ ", format.Green, useColor)
	}
}

// ---------- INSIGHTS SUB-VIEW ----------

// renderInsightsSubView shows full insights: hourly distribution bars with
// peak-hour markers, model distribution table, and efficiency metrics.
func renderInsightsSubView(state *dashState) string {
	if state.data == nil || len(state.data.entries) == 0 {
		return padLine("  No usage data.")
	}

	entries := state.data.entries
	useColor := state.useColor

	hourly := usage.AggregateHourly(entries)
	models := usage.AggregateModelDistribution(entries)
	eff := usage.CalculateEfficiency(entries)
	peaks := usage.FindPeakHours(entries, 3)

	// Build a set of peak hours for marking.
	peakSet := make(map[int]bool)
	for _, p := range peaks {
		peakSet[p.Hour] = true
	}

	var b strings.Builder

	// --- Hourly distribution ---
	b.WriteString(padLine("  " + format.Colorize("Hourly Distribution (UTC)", format.Bold+format.White, useColor)))
	b.WriteString(emptyLine())

	// Find max tokens for bar scaling.
	var maxTokens int64
	for _, h := range hourly {
		if h.Summary.TotalTokens > maxTokens {
			maxTokens = h.Summary.TotalTokens
		}
	}

	const hourBarWidth = 20
	for _, h := range hourly {
		barLen := 0
		if maxTokens > 0 {
			barLen = int(float64(h.Summary.TotalTokens) / float64(maxTokens) * hourBarWidth)
		}

		bar := strings.Repeat("█", barLen)
		barColor := format.Green
		pct := float64(barLen) / hourBarWidth * 100
		if pct > 80 {
			barColor = format.Red
		} else if pct > 50 {
			barColor = format.Yellow
		}

		peakMark := "  "
		if peakSet[h.Hour] {
			peakMark = format.Colorize("★ ", format.Yellow, useColor)
		}

		line := fmt.Sprintf("  %s%s  %s  %s",
			peakMark,
			format.Colorize(fmt.Sprintf("%02d:00", h.Hour), format.Cyan, useColor),
			format.Colorize(bar, barColor, useColor),
			format.Colorize(fmt.Sprintf("$%.2f", h.Summary.CostUSD), format.Dim, useColor))
		b.WriteString(padLine(line))
	}

	b.WriteString(emptyLine())

	// --- Model distribution ---
	b.WriteString(padLine("  " + format.Colorize("Model Distribution", format.Bold+format.White, useColor)))
	b.WriteString(emptyLine())

	for _, m := range models {
		name := shortModelName(m.Model)
		costStr := fmt.Sprintf("$%.2f", m.Summary.CostUSD)
		tokenStr := format.FormatNumber(m.Summary.TotalTokens)
		shareStr := fmt.Sprintf("%.1f%%", m.CostPercent)

		line := fmt.Sprintf("  %-10s %12s tok  %8s  %s",
			format.Colorize(name, format.Cyan, useColor),
			tokenStr,
			format.Colorize(costStr, format.Yellow, useColor),
			format.Colorize(shareStr, format.Dim, useColor))
		b.WriteString(padLine(line))
	}

	b.WriteString(emptyLine())

	// --- Efficiency metrics ---
	b.WriteString(padLine("  " + format.Colorize("Efficiency", format.Bold+format.White, useColor)))
	b.WriteString(emptyLine())

	cacheColor := format.Yellow
	if eff.CacheHitRatio > 0.5 {
		cacheColor = format.Green
	}

	b.WriteString(padLine(fmt.Sprintf("  Cache hit ratio:    %s",
		format.Colorize(fmt.Sprintf("%.1f%%", eff.CacheHitRatio*100), cacheColor, useColor))))
	b.WriteString(padLine(fmt.Sprintf("  Avg tokens/msg:     %s",
		format.FormatNumber(eff.AvgTokensPerMsg))))
	b.WriteString(padLine(fmt.Sprintf("  Avg cost/msg:       %s",
		format.Colorize(fmt.Sprintf("$%.4f", eff.AvgCostPerMsg), format.Dim, useColor))))
	b.WriteString(padLine(fmt.Sprintf("  Input/output ratio: %.1f:1",
		eff.InputOutputRatio)))

	return b.String()
}

// ---------- ROI SUB-VIEW ----------

// renderROISubView shows a monthly breakdown table with API cost,
// subscription cost, and savings per month, plus a lifetime total row.
func renderROISubView(state *dashState) string {
	if state.data == nil || len(state.data.entries) == 0 {
		return padLine("  No usage data.")
	}

	useColor := state.useColor
	monthly := usage.AggregateMonthly(state.data.entries)
	subCost := totalSubscriptionCost()

	if len(monthly) == 0 {
		return padLine("  No monthly data.")
	}

	var b strings.Builder

	// Header.
	header := fmt.Sprintf("  %-10s %12s %12s %12s %8s",
		"Month", "API Cost", "Subscription", "Savings", "ROI")
	b.WriteString(padLine(format.Colorize(header, format.Bold+format.White, useColor)))
	sep := "  " + strings.Repeat("─", dashboardBoxWidth-4)
	b.WriteString(padLine(format.Colorize(sep, format.Dim, useColor)))

	var totalAPI float64
	var totalSub float64

	for _, m := range monthly {
		apiCost := m.Summary.CostUSD
		savings := apiCost - subCost
		var roiPct float64
		if apiCost > 0 {
			roiPct = savings / apiCost * 100
		}

		savingsColor := format.Green
		if savings < 0 {
			savingsColor = format.Red
		}

		line := fmt.Sprintf("  %-10s %12s %12s %12s %7.0f%%",
			format.Colorize(m.Month, format.Cyan, useColor),
			format.Colorize(fmt.Sprintf("$%.2f", apiCost), format.Yellow, useColor),
			format.Colorize(fmt.Sprintf("$%.0f", subCost), format.Dim, useColor),
			format.Colorize(fmt.Sprintf("$%.2f", savings), savingsColor, useColor),
			roiPct)
		b.WriteString(padLine(line))

		totalAPI += apiCost
		totalSub += subCost
	}

	// Total row.
	totalSavings := totalAPI - totalSub
	var totalROI float64
	if totalAPI > 0 {
		totalROI = totalSavings / totalAPI * 100
	}

	savingsColor := format.Green
	if totalSavings < 0 {
		savingsColor = format.Red
	}

	b.WriteString(padLine(format.Colorize(sep, format.Dim, useColor)))
	totalLine := fmt.Sprintf("  %-10s %12s %12s %12s %7.0f%%",
		format.Colorize("Lifetime", format.Bold+format.White, useColor),
		format.Colorize(fmt.Sprintf("$%.2f", totalAPI), format.Yellow+format.Bold, useColor),
		format.Colorize(fmt.Sprintf("$%.0f", totalSub), format.Dim, useColor),
		format.Colorize(fmt.Sprintf("$%.2f", totalSavings), savingsColor+format.Bold, useColor),
		totalROI)
	b.WriteString(padLine(totalLine))

	return b.String()
}
