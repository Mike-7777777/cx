package statusline

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/MasaYan24/cc-monitor/internal/format"
)

const barWidth = 5

// OtherAccount holds rate-limit info for a secondary account.
type OtherAccount struct {
	Name     string
	FiveHour float64
	SevenDay float64
	Stale    string // "15m", "", or "reset"
}

// Render produces one or two status-line strings.
//
// Line 1: [Opus 4.6] 32% ctx | 5h: ██░░░ 38% (2h14m) | 7d: ███░░ 62%
// Line 2 (if other != nil): [5x] 5h: ████░ 71% | stale 15m
//
// When no rate_limits: [Opus 4.6] 32% ctx
// When context used_percentage is nil: treat as 0.
func Render(input *Input, other *OtherAccount, useColor bool) []string {
	var lines []string
	lines = append(lines, renderPrimary(input, useColor))
	if other != nil {
		lines = append(lines, renderOther(other, useColor))
	}
	return lines
}

func renderPrimary(input *Input, useColor bool) string {
	ctxPct := 0.0
	if input.ContextWindow.UsedPercentage != nil {
		ctxPct = *input.ContextWindow.UsedPercentage
	}

	modelName := format.Colorize(input.Model.DisplayName, format.Cyan+format.Bold, useColor)
	ctxStr := format.Colorize(fmt.Sprintf("%d%%", int(ctxPct)), format.UsageColor(ctxPct), useColor)

	line := fmt.Sprintf("[%s] %s ctx", modelName, ctxStr)

	if input.RateLimits == nil {
		return line
	}

	if input.RateLimits.FiveHour != nil {
		w := input.RateLimits.FiveHour
		bar := colorProgressBar(w.UsedPercentage, barWidth, useColor)
		pctStr := format.Colorize(fmt.Sprintf("%d%%", int(math.Round(w.UsedPercentage))), format.UsageColor(w.UsedPercentage), useColor)
		ttl := format.FormatDuration(time.Until(time.Unix(w.ResetsAt, 0)))
		line += fmt.Sprintf(" | 5h: %s %s (%s)", bar, pctStr, ttl)
	}

	if input.RateLimits.SevenDay != nil {
		w := input.RateLimits.SevenDay
		bar := colorProgressBar(w.UsedPercentage, barWidth, useColor)
		pctStr := format.Colorize(fmt.Sprintf("%d%%", int(math.Round(w.UsedPercentage))), format.UsageColor(w.UsedPercentage), useColor)
		line += fmt.Sprintf(" | 7d: %s %s", bar, pctStr)
	}

	return line
}

func renderOther(other *OtherAccount, useColor bool) string {
	bar5h := colorProgressBar(other.FiveHour, barWidth, useColor)
	pct5h := format.Colorize(fmt.Sprintf("%d%%", int(math.Round(other.FiveHour))), format.UsageColor(other.FiveHour), useColor)

	nameStr := format.Colorize(other.Name, format.Cyan, useColor)
	line := fmt.Sprintf("[%s] 5h: %s %s", nameStr, bar5h, pct5h)

	if other.SevenDay > 0 {
		bar7d := colorProgressBar(other.SevenDay, barWidth, useColor)
		pct7d := format.Colorize(fmt.Sprintf("%d%%", int(math.Round(other.SevenDay))), format.UsageColor(other.SevenDay), useColor)
		line += fmt.Sprintf(" | 7d: %s %s", bar7d, pct7d)
	}

	if other.Stale != "" {
		if other.Stale == "reset" {
			line += " | " + format.Colorize("reset", format.Green, useColor)
		} else {
			line += " | " + format.Colorize(fmt.Sprintf("stale %s", other.Stale), format.Dim, useColor)
		}
	}

	return line
}

// colorProgressBar renders a progress bar where filled blocks are colored by
// percentage and empty blocks are dimmed when color is enabled.
func colorProgressBar(pct float64, width int, useColor bool) string {
	plain := format.ProgressBar(pct, width)
	if !useColor {
		return plain
	}

	filledCount := 0
	for _, r := range plain {
		if r == '█' {
			filledCount++
		}
	}
	emptyCount := width - filledCount

	fillColor := format.UsageColor(pct)
	filled := format.Colorize(strings.Repeat("█", filledCount), fillColor, useColor)
	empty := format.Colorize(strings.Repeat("░", emptyCount), format.Dim, useColor)
	return filled + empty
}
