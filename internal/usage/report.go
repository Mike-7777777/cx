package usage

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Mike-7777777/cx/internal/format"
)

// formatCost formats a USD cost as "$X.XX".
func formatCost(cost float64) string {
	return fmt.Sprintf("$%.2f", cost)
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
			format.FormatNumber(r.Summary.InputTokens),
			format.FormatNumber(r.Summary.OutputTokens),
			format.FormatNumber(r.Summary.CacheReadInputTokens),
			format.FormatNumber(r.Summary.CacheCreationInputTokens),
			format.FormatNumber(r.Summary.TotalTokens),
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
		format.FormatNumber(total.InputTokens),
		format.FormatNumber(total.OutputTokens),
		format.FormatNumber(total.CacheReadInputTokens),
		format.FormatNumber(total.CacheCreationInputTokens),
		format.FormatNumber(total.TotalTokens),
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
			format.FormatDuration(dur),
			format.FormatNumber(r.Summary.TotalTokens),
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
		format.FormatNumber(totalTokens),
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
			format.FormatNumber(r.Summary.TotalTokens),
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
		format.FormatNumber(totalTokens),
		formatCost(totalCost),
	)
	b.WriteString(format.Colorize(totalRow, format.Bold, useColor))

	return b.String()
}

// FormatDailyTableWithBreakdown formats daily reports with per-model subtotals.
func FormatDailyTableWithBreakdown(reports []DailyReport, useColor bool) string {
	const (
		fmtRow    = "%-13s %11s %11s %11s %13s %11s %9s\n"
		fmtSubRow = "  %-11s %11s %11s %11s %13s %11s %9s\n"
		width     = 82
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
			format.FormatNumber(r.Summary.InputTokens),
			format.FormatNumber(r.Summary.OutputTokens),
			format.FormatNumber(r.Summary.CacheReadInputTokens),
			format.FormatNumber(r.Summary.CacheCreationInputTokens),
			format.FormatNumber(r.Summary.TotalTokens),
			costStr,
		))
		total.InputTokens += r.Summary.InputTokens
		total.OutputTokens += r.Summary.OutputTokens
		total.CacheReadInputTokens += r.Summary.CacheReadInputTokens
		total.CacheCreationInputTokens += r.Summary.CacheCreationInputTokens
		total.TotalTokens += r.Summary.TotalTokens
		total.CostUSD += r.Summary.CostUSD

		// Per-model breakdown rows
		models := sortedModelNames(r.Models)
		for _, model := range models {
			ms := r.Models[model]
			modelStr := format.Colorize(model, format.Dim, useColor)
			mCostStr := format.Colorize(formatCost(ms.CostUSD), format.Dim, useColor)
			b.WriteString(fmt.Sprintf(fmtSubRow,
				modelStr,
				format.FormatNumber(ms.InputTokens),
				format.FormatNumber(ms.OutputTokens),
				format.FormatNumber(ms.CacheReadInputTokens),
				format.FormatNumber(ms.CacheCreationInputTokens),
				format.FormatNumber(ms.TotalTokens),
				mCostStr,
			))
		}
	}

	b.WriteString(repeatSep(width, useColor))
	b.WriteString("\n")

	totalRow := fmt.Sprintf(fmtRow,
		"Total",
		format.FormatNumber(total.InputTokens),
		format.FormatNumber(total.OutputTokens),
		format.FormatNumber(total.CacheReadInputTokens),
		format.FormatNumber(total.CacheCreationInputTokens),
		format.FormatNumber(total.TotalTokens),
		formatCost(total.CostUSD),
	)
	b.WriteString(format.Colorize(totalRow, format.Bold, useColor))

	return b.String()
}

// FormatMonthlyTable formats monthly reports as a human-readable table.
func FormatMonthlyTable(reports []MonthlyReport, useColor bool) string {
	const (
		fmtRow = "%-10s %11s %11s %11s %13s %11s %9s\n"
		width  = 79
	)

	var b strings.Builder

	header := fmt.Sprintf(fmtRow, "Month", "Input", "Output", "Cache Read", "Cache Create", "Total", "Cost")
	b.WriteString(format.Colorize(header, format.Bold, useColor))
	b.WriteString(repeatSep(width, useColor))
	b.WriteString("\n")

	var total UsageSummary
	for _, r := range reports {
		monthStr := format.Colorize(r.Month, format.Cyan, useColor)
		costStr := format.Colorize(formatCost(r.Summary.CostUSD), costColor(r.Summary.CostUSD), useColor)
		b.WriteString(fmt.Sprintf(fmtRow,
			monthStr,
			format.FormatNumber(r.Summary.InputTokens),
			format.FormatNumber(r.Summary.OutputTokens),
			format.FormatNumber(r.Summary.CacheReadInputTokens),
			format.FormatNumber(r.Summary.CacheCreationInputTokens),
			format.FormatNumber(r.Summary.TotalTokens),
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
		format.FormatNumber(total.InputTokens),
		format.FormatNumber(total.OutputTokens),
		format.FormatNumber(total.CacheReadInputTokens),
		format.FormatNumber(total.CacheCreationInputTokens),
		format.FormatNumber(total.TotalTokens),
		formatCost(total.CostUSD),
	)
	b.WriteString(format.Colorize(totalRow, format.Bold, useColor))

	return b.String()
}

// FormatWeeklyTable formats weekly reports as a human-readable table.
func FormatWeeklyTable(reports []WeeklyReport, useColor bool) string {
	const (
		fmtRow = "%-12s %11s %11s %11s %13s %11s %9s\n"
		width  = 81
	)

	var b strings.Builder

	header := fmt.Sprintf(fmtRow, "Week", "Input", "Output", "Cache Read", "Cache Create", "Total", "Cost")
	b.WriteString(format.Colorize(header, format.Bold, useColor))
	b.WriteString(repeatSep(width, useColor))
	b.WriteString("\n")

	var total UsageSummary
	for _, r := range reports {
		weekStr := format.Colorize(r.Week, format.Cyan, useColor)
		costStr := format.Colorize(formatCost(r.Summary.CostUSD), costColor(r.Summary.CostUSD), useColor)
		b.WriteString(fmt.Sprintf(fmtRow,
			weekStr,
			format.FormatNumber(r.Summary.InputTokens),
			format.FormatNumber(r.Summary.OutputTokens),
			format.FormatNumber(r.Summary.CacheReadInputTokens),
			format.FormatNumber(r.Summary.CacheCreationInputTokens),
			format.FormatNumber(r.Summary.TotalTokens),
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
		format.FormatNumber(total.InputTokens),
		format.FormatNumber(total.OutputTokens),
		format.FormatNumber(total.CacheReadInputTokens),
		format.FormatNumber(total.CacheCreationInputTokens),
		format.FormatNumber(total.TotalTokens),
		formatCost(total.CostUSD),
	)
	b.WriteString(format.Colorize(totalRow, format.Bold, useColor))

	return b.String()
}

// FormatMessagesTable formats individual entries as a per-message table.
// Entries should be pre-sorted (typically timestamp descending, newest first).
func FormatMessagesTable(entries []Entry, useColor bool) string {
	const (
		fmtRow = "%-21s %-22s %9s %9s %12s %13s %11s %9s\n"
		width  = 110
	)

	var b strings.Builder

	header := fmt.Sprintf(fmtRow, "Timestamp", "Model", "Input", "Output", "Cache Read", "Cache Create", "Total", "Cost")
	b.WriteString(format.Colorize(header, format.Bold, useColor))
	b.WriteString(repeatSep(width, useColor))
	b.WriteString("\n")

	var totalTokens int64
	var totalCost float64
	for _, e := range entries {
		total := e.Usage.InputTokens + e.Usage.OutputTokens +
			e.Usage.CacheCreationInputTokens + e.Usage.CacheReadInputTokens
		cost := CalculateCost(e.Model, e.Usage)

		tsStr := format.Colorize(e.Timestamp.UTC().Format("2006-01-02 15:04"), format.Cyan, useColor)
		costStr := format.Colorize(formatCost(cost), costColor(cost), useColor)
		modelStr := truncateModel(e.Model)

		b.WriteString(fmt.Sprintf(fmtRow,
			tsStr,
			modelStr,
			format.FormatNumber(e.Usage.InputTokens),
			format.FormatNumber(e.Usage.OutputTokens),
			format.FormatNumber(e.Usage.CacheReadInputTokens),
			format.FormatNumber(e.Usage.CacheCreationInputTokens),
			format.FormatNumber(total),
			costStr,
		))
		totalTokens += total
		totalCost += cost
	}

	b.WriteString(repeatSep(width, useColor))
	b.WriteString("\n")

	totalRow := fmt.Sprintf(fmtRow,
		fmt.Sprintf("Total (%d msgs)", len(entries)),
		"",
		"",
		"",
		"",
		"",
		format.FormatNumber(totalTokens),
		formatCost(totalCost),
	)
	b.WriteString(format.Colorize(totalRow, format.Bold, useColor))

	return b.String()
}

// truncateModel shortens a model name for display (max 22 chars).
func truncateModel(model string) string {
	if len(model) <= 22 {
		return model
	}
	return model[:19] + "..."
}

// FormatProjectTable formats project usage reports as a table.
func FormatProjectTable(reports []ProjectReport, useColor bool) string {
	const (
		fmtRow = "%-38s %11s %9s %5s\n"
		width  = 66
	)

	var b strings.Builder

	header := fmt.Sprintf(fmtRow, "Project", "Tokens", "Cost", "Msgs")
	b.WriteString(format.Colorize(header, format.Bold, useColor))
	b.WriteString(repeatSep(width, useColor))
	b.WriteString("\n")

	var totalTokens int64
	var totalCost float64
	var totalMsgs int
	for _, r := range reports {
		projStr := truncateProject(r.Project)
		projStr = format.Colorize(projStr, format.Cyan, useColor)
		costStr := format.Colorize(formatCost(r.Summary.CostUSD), costColor(r.Summary.CostUSD), useColor)
		b.WriteString(fmt.Sprintf(fmtRow,
			projStr,
			format.FormatNumber(r.Summary.TotalTokens),
			costStr,
			fmt.Sprintf("%d", r.Summary.EntryCount),
		))
		totalTokens += r.Summary.TotalTokens
		totalCost += r.Summary.CostUSD
		totalMsgs += r.Summary.EntryCount
	}

	b.WriteString(repeatSep(width, useColor))
	b.WriteString("\n")

	totalRow := fmt.Sprintf(fmtRow,
		"Total",
		format.FormatNumber(totalTokens),
		formatCost(totalCost),
		fmt.Sprintf("%d", totalMsgs),
	)
	b.WriteString(format.Colorize(totalRow, format.Bold, useColor))

	return b.String()
}

// truncateProject shortens a project path for display (max 36 chars).
func truncateProject(proj string) string {
	if len(proj) <= 36 {
		return proj
	}
	return proj[:33] + "..."
}

// FormatTrendTable formats trend comparison pairs as a table.
func FormatTrendTable(pairs []TrendPair, useColor bool) string {
	const (
		fmtRow = "%-13s %18s %18s %9s %11s %11s %9s\n"
		width  = 93
	)

	var b strings.Builder

	header := fmt.Sprintf(fmtRow, "Date", "Tokens (current)", "Tokens (previous)", "Change", "Cost (cur)", "Cost (prev)", "Change")
	b.WriteString(format.Colorize(header, format.Bold, useColor))
	b.WriteString(repeatSep(width, useColor))
	b.WriteString("\n")

	for _, p := range pairs {
		isTotal := p.Date == "Total"

		changeStr := formatChange(p.ChangePercent)
		costChangeStr := formatChange(p.CostChangePct)

		if useColor {
			changeStr = format.Colorize(changeStr, changeColor(p.ChangePercent), useColor)
			costChangeStr = format.Colorize(costChangeStr, changeColor(p.CostChangePct), useColor)
		}

		dateStr := p.Date
		if !isTotal {
			dateStr = format.Colorize(p.Date, format.Cyan, useColor)
		}

		row := fmt.Sprintf(fmtRow,
			dateStr,
			format.FormatNumber(p.CurrentTokens),
			format.FormatNumber(p.PreviousTokens),
			changeStr,
			formatCost(p.CurrentCost),
			formatCost(p.PreviousCost),
			costChangeStr,
		)

		if isTotal {
			b.WriteString(repeatSep(width, useColor))
			b.WriteString("\n")
			b.WriteString(format.Colorize(row, format.Bold, useColor))
		} else {
			b.WriteString(row)
		}
	}

	return b.String()
}

// formatChange formats a percentage change as "+X.X%" or "-X.X%".
func formatChange(pct float64) string {
	if pct == 0 {
		return "0.0%"
	}
	if pct > 0 {
		return fmt.Sprintf("+%.1f%%", pct)
	}
	return fmt.Sprintf("%.1f%%", pct)
}

// changeColor returns Red for increases, Green for decreases (lower cost is good).
func changeColor(pct float64) string {
	if pct > 0 {
		return format.Red
	}
	if pct < 0 {
		return format.Green
	}
	return format.Dim
}

// FormatSubagentBreakdown formats the subagent vs main session cost split.
func FormatSubagentBreakdown(b SubagentBreakdown, useColor bool) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(format.Colorize("Subagent Breakdown\n", format.Bold, useColor))
	sb.WriteString(repeatSep(40, useColor))
	sb.WriteString("\n")

	total := b.MainCost + b.SubagentCost
	var mainPct, subPct float64
	if total > 0 {
		mainPct = b.MainCost / total * 100
		subPct = b.SubagentCost / total * 100
	}

	mainCostStr := format.Colorize(formatCost(b.MainCost), format.Green, useColor)
	subCostStr := format.Colorize(formatCost(b.SubagentCost), format.Yellow, useColor)

	sb.WriteString(fmt.Sprintf("  Main sessions:  %s (%4.1f%%)  [%s tokens, %d msgs]\n",
		mainCostStr, mainPct, format.FormatNumber(b.MainTokens), b.MainCount))
	sb.WriteString(fmt.Sprintf("  Subagents:      %s (%4.1f%%)  [%s tokens, %d msgs]\n",
		subCostStr, subPct, format.FormatNumber(b.SubagentTokens), b.SubagentCount))

	return sb.String()
}

// FormatDailyCSV formats daily reports as CSV.
func FormatDailyCSV(reports []DailyReport) string {
	var b strings.Builder
	b.WriteString("Date,Input,Output,CacheRead,CacheCreate,Total,Cost\n")
	for _, r := range reports {
		b.WriteString(fmt.Sprintf("%s,%d,%d,%d,%d,%d,%.2f\n",
			r.Date,
			r.Summary.InputTokens,
			r.Summary.OutputTokens,
			r.Summary.CacheReadInputTokens,
			r.Summary.CacheCreationInputTokens,
			r.Summary.TotalTokens,
			r.Summary.CostUSD,
		))
	}
	return b.String()
}

// FormatDailyMarkdown formats daily reports as a GitHub-flavored markdown table.
func FormatDailyMarkdown(reports []DailyReport) string {
	var b strings.Builder
	b.WriteString("| Date | Input | Output | Cache Read | Cache Create | Total | Cost |\n")
	b.WriteString("|------|------:|-------:|-----------:|-------------:|------:|-----:|\n")
	for _, r := range reports {
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s |\n",
			r.Date,
			format.FormatNumber(r.Summary.InputTokens),
			format.FormatNumber(r.Summary.OutputTokens),
			format.FormatNumber(r.Summary.CacheReadInputTokens),
			format.FormatNumber(r.Summary.CacheCreationInputTokens),
			format.FormatNumber(r.Summary.TotalTokens),
			formatCost(r.Summary.CostUSD),
		))
	}
	return b.String()
}

// FormatROI formats the ROI summary section.
func FormatROI(totalCostUSD float64, subscriptionCost float64, useColor bool) string {
	savings := totalCostUSD - subscriptionCost
	var roiPct float64
	if totalCostUSD > 0 {
		roiPct = savings / totalCostUSD * 100
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(format.Colorize("ROI Summary\n", format.Bold, useColor))
	b.WriteString(fmt.Sprintf("  Subscription cost:   %s/month\n", formatCost(subscriptionCost)))
	b.WriteString(fmt.Sprintf("  Equivalent API cost: %s\n", format.Colorize(formatCost(totalCostUSD), format.Yellow, useColor)))
	b.WriteString(fmt.Sprintf("  Savings:             %s (%.1f%%)\n",
		format.Colorize(formatCost(savings), format.Green, useColor),
		roiPct,
	))
	return b.String()
}

// sortedModelNames returns model names sorted alphabetically.
func sortedModelNames(models map[string]*UsageSummary) []string {
	names := make([]string, 0, len(models))
	for name := range models {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// CLIToolReport summarizes usage attributed to one AI CLI tool.
type CLIToolReport struct {
	Tool    string       `json:"Tool"`
	Summary UsageSummary `json:"Summary"`
}

// AggregateByCLITool groups entries by CLITool name.
// Returns results sorted by total cost descending.
func AggregateByCLITool(entries []Entry) []CLIToolReport {
	byTool := make(map[string]*UsageSummary)
	for _, e := range entries {
		tool := e.CLITool
		if tool == "" {
			tool = "claude-code"
		}
		s, ok := byTool[tool]
		if !ok {
			s = &UsageSummary{}
			byTool[tool] = s
		}
		addEntry(s, e)
	}

	reports := make([]CLIToolReport, 0, len(byTool))
	for tool, s := range byTool {
		reports = append(reports, CLIToolReport{Tool: tool, Summary: *s})
	}
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Summary.CostUSD > reports[j].Summary.CostUSD
	})
	return reports
}

// FormatCLIToolTable formats CLI tool usage as a summary table.
func FormatCLIToolTable(reports []CLIToolReport, useColor bool) string {
	const (
		fmtRow = "%-16s %11s %9s %5s\n"
		width  = 45
	)

	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(format.Colorize("By CLI Tool\n", format.Bold, useColor))
	header := fmt.Sprintf(fmtRow, "Tool", "Tokens", "Cost", "Msgs")
	b.WriteString(format.Colorize(header, format.Bold, useColor))
	b.WriteString(repeatSep(width, useColor))
	b.WriteString("\n")

	var totalTokens int64
	var totalCost float64
	var totalMsgs int
	for _, r := range reports {
		toolStr := format.Colorize(r.Tool, format.Cyan, useColor)
		costStr := format.Colorize(formatCost(r.Summary.CostUSD), costColor(r.Summary.CostUSD), useColor)
		b.WriteString(fmt.Sprintf(fmtRow,
			toolStr,
			format.FormatNumber(r.Summary.TotalTokens),
			costStr,
			fmt.Sprintf("%d", r.Summary.EntryCount),
		))
		totalTokens += r.Summary.TotalTokens
		totalCost += r.Summary.CostUSD
		totalMsgs += r.Summary.EntryCount
	}

	b.WriteString(repeatSep(width, useColor))
	b.WriteString("\n")

	totalRow := fmt.Sprintf(fmtRow,
		"Total",
		format.FormatNumber(totalTokens),
		formatCost(totalCost),
		fmt.Sprintf("%d", totalMsgs),
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
