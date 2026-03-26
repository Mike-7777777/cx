package usage

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Mike-7777777/cx/internal/format"
)

// HourlyReport summarizes usage for a single UTC hour (0–23).
type HourlyReport struct {
	Hour    int          `json:"Hour"`
	Summary UsageSummary `json:"Summary"`
}

// AggregateHourly groups entries by UTC hour of day (0–23).
// Returns a slice sorted by hour ascending.
func AggregateHourly(entries []Entry) []HourlyReport {
	byHour := make(map[int]*UsageSummary)

	for _, e := range entries {
		h := e.Timestamp.UTC().Hour()
		s, ok := byHour[h]
		if !ok {
			s = &UsageSummary{}
			byHour[h] = s
		}
		addEntry(s, e)
	}

	reports := make([]HourlyReport, 0, len(byHour))
	for h, s := range byHour {
		reports = append(reports, HourlyReport{Hour: h, Summary: *s})
	}
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Hour < reports[j].Hour
	})
	return reports
}

// ModelDistributionReport summarizes usage for a single model with percentage shares.
type ModelDistributionReport struct {
	Model        string       `json:"Model"`
	Summary      UsageSummary `json:"Summary"`
	CostPercent  float64      `json:"CostPercent"`
	TokenPercent float64      `json:"TokenPercent"`
	MsgPercent   float64      `json:"MsgPercent"`
}

// AggregateModelDistribution groups entries by model and computes percentage
// shares of total cost, tokens, and message count.
// Returns a slice sorted by cost descending (highest cost model first).
func AggregateModelDistribution(entries []Entry) []ModelDistributionReport {
	byModel := make(map[string]*UsageSummary)

	for _, e := range entries {
		s, ok := byModel[e.Model]
		if !ok {
			s = &UsageSummary{}
			byModel[e.Model] = s
		}
		addEntry(s, e)
	}

	// Compute totals for percentage calculation.
	var totalCost float64
	var totalTokens int64
	var totalMsgs int
	for _, s := range byModel {
		totalCost += s.CostUSD
		totalTokens += s.TotalTokens
		totalMsgs += s.EntryCount
	}

	reports := make([]ModelDistributionReport, 0, len(byModel))
	for model, s := range byModel {
		var costPct, tokenPct, msgPct float64
		if totalCost > 0 {
			costPct = s.CostUSD / totalCost * 100
		}
		if totalTokens > 0 {
			tokenPct = float64(s.TotalTokens) / float64(totalTokens) * 100
		}
		if totalMsgs > 0 {
			msgPct = float64(s.EntryCount) / float64(totalMsgs) * 100
		}
		reports = append(reports, ModelDistributionReport{
			Model:        model,
			Summary:      *s,
			CostPercent:  costPct,
			TokenPercent: tokenPct,
			MsgPercent:   msgPct,
		})
	}

	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Summary.CostUSD > reports[j].Summary.CostUSD
	})
	return reports
}

// EfficiencyMetrics holds derived efficiency indicators across a set of entries.
type EfficiencyMetrics struct {
	// CacheHitRatio is cache_reads / (cache_reads + cache_creates). Zero when no cache tokens.
	CacheHitRatio float64 `json:"CacheHitRatio"`
	// AvgTokensPerMsg is total tokens / entry count. Zero when no entries.
	AvgTokensPerMsg int64 `json:"AvgTokensPerMsg"`
	// AvgCostPerMsg is total cost / entry count. Zero when no entries.
	AvgCostPerMsg float64 `json:"AvgCostPerMsg"`
	// InputOutputRatio is input tokens / output tokens. Zero when no output tokens.
	InputOutputRatio float64 `json:"InputOutputRatio"`
}

// CalculateEfficiency computes efficiency metrics across all entries.
func CalculateEfficiency(entries []Entry) EfficiencyMetrics {
	var totalTokens, inputTokens, outputTokens, cacheReads, cacheCreates int64
	var totalCost float64

	for _, e := range entries {
		inputTokens += e.Usage.InputTokens
		outputTokens += e.Usage.OutputTokens
		cacheReads += e.Usage.CacheReadInputTokens
		cacheCreates += e.Usage.CacheCreationInputTokens
		totalTokens += e.Usage.InputTokens + e.Usage.OutputTokens +
			e.Usage.CacheCreationInputTokens + e.Usage.CacheReadInputTokens
		totalCost += CalculateCost(e.Model, e.Usage)
	}

	n := int64(len(entries))
	var m EfficiencyMetrics

	cacheTotal := cacheReads + cacheCreates
	if cacheTotal > 0 {
		m.CacheHitRatio = float64(cacheReads) / float64(cacheTotal)
	}
	if n > 0 {
		m.AvgTokensPerMsg = totalTokens / n
		m.AvgCostPerMsg = totalCost / float64(n)
	}
	if outputTokens > 0 {
		m.InputOutputRatio = float64(inputTokens) / float64(outputTokens)
	}

	return m
}

// FormatHourlyTable formats hourly distribution reports as a human-readable table
// with an activity bar chart scaled to the maximum token count across all hours.
func FormatHourlyTable(reports []HourlyReport, useColor bool) string {
	const (
		fmtRow  = "%-6s %6s %12s %10s  %-20s\n"
		width   = 60
		maxBars = 20
	)

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(format.Colorize("Hourly Distribution (UTC)\n", format.Bold, useColor))

	header := fmt.Sprintf(fmtRow, "Hour", "Msgs", "Tokens", "Cost", "Activity")
	b.WriteString(format.Colorize(header, format.Bold, useColor))
	b.WriteString(repeatSep(width, useColor))
	b.WriteString("\n")

	// Find max token count to scale bar chart.
	var maxTokens int64
	for _, r := range reports {
		if r.Summary.TotalTokens > maxTokens {
			maxTokens = r.Summary.TotalTokens
		}
	}

	for _, r := range reports {
		hourStr := format.Colorize(fmt.Sprintf("%02d:00", r.Hour), format.Cyan, useColor)
		costStr := format.Colorize(formatCost(r.Summary.CostUSD), costColor(r.Summary.CostUSD), useColor)

		barLen := 0
		if maxTokens > 0 {
			barLen = int(float64(r.Summary.TotalTokens) / float64(maxTokens) * maxBars)
		}
		pct := float64(barLen) / maxBars * 100
		bar := strings.Repeat("█", barLen)
		barStr := format.Colorize(bar, format.UsageColor(pct), useColor)

		b.WriteString(fmt.Sprintf(fmtRow,
			hourStr,
			fmt.Sprintf("%d", r.Summary.EntryCount),
			format.FormatNumber(r.Summary.TotalTokens),
			costStr,
			barStr,
		))
	}

	return b.String()
}

// FormatModelDistributionTable formats model distribution reports as a table
// with a bar chart scaled to the token share percentage.
func FormatModelDistributionTable(reports []ModelDistributionReport, useColor bool) string {
	const (
		fmtRow  = "%-22s %6s %12s %10s %7s  %-20s\n"
		width   = 83
		maxBars = 20
	)

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(format.Colorize("Model Distribution\n", format.Bold, useColor))

	header := fmt.Sprintf(fmtRow, "Model", "Msgs", "Tokens", "Cost", "Share", "")
	b.WriteString(format.Colorize(header, format.Bold, useColor))
	b.WriteString(repeatSep(width, useColor))
	b.WriteString("\n")

	for _, r := range reports {
		modelStr := format.Colorize(truncateModel(r.Model), format.Cyan, useColor)
		costStr := format.Colorize(formatCost(r.Summary.CostUSD), costColor(r.Summary.CostUSD), useColor)
		shareStr := fmt.Sprintf("%.1f%%", r.CostPercent)

		barLen := min(int(r.CostPercent/5), maxBars)
		bar := strings.Repeat("█", barLen)
		barStr := format.Colorize(bar, format.UsageColor(r.CostPercent), useColor)

		b.WriteString(fmt.Sprintf(fmtRow,
			modelStr,
			fmt.Sprintf("%d", r.Summary.EntryCount),
			format.FormatNumber(r.Summary.TotalTokens),
			costStr,
			shareStr,
			barStr,
		))
	}

	return b.String()
}

// FormatEfficiency formats efficiency metrics as a labeled section.
func FormatEfficiency(m EfficiencyMetrics, useColor bool) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(format.Colorize("Efficiency Metrics\n", format.Bold, useColor))
	b.WriteString(repeatSep(40, useColor))
	b.WriteString("\n")

	cacheColor := format.Yellow
	if m.CacheHitRatio > 0.5 {
		cacheColor = format.Green
	}
	cacheStr := format.Colorize(fmt.Sprintf("%.1f%%", m.CacheHitRatio*100), cacheColor, useColor)
	b.WriteString(fmt.Sprintf("  Cache hit ratio:      %s\n", cacheStr))
	b.WriteString(fmt.Sprintf("  Avg tokens/message:   %s\n", format.FormatNumber(m.AvgTokensPerMsg)))
	b.WriteString(fmt.Sprintf("  Avg cost/message:     %s\n", formatCost(m.AvgCostPerMsg)))
	b.WriteString(fmt.Sprintf("  Input/output ratio:   %.1f:1\n", m.InputOutputRatio))

	return b.String()
}

// FindPeakHours returns the top n hourly reports ranked by total token count
// in descending order. If n >= number of distinct hours, all hours are returned.
// If n <= 0, an empty slice is returned.
func FindPeakHours(entries []Entry, n int) []HourlyReport {
	all := AggregateHourly(entries)

	// Sort by total tokens descending.
	sort.Slice(all, func(i, j int) bool {
		return all[i].Summary.TotalTokens > all[j].Summary.TotalTokens
	})

	if n <= 0 {
		return nil
	}
	if n > len(all) {
		n = len(all)
	}
	return all[:n]
}
