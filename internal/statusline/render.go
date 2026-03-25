package statusline

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Mike-7777777/cc-monitor/internal/format"
)

const barWidth = 5

// OtherAccount holds rate-limit info for a secondary account.
type OtherAccount struct {
	Name     string
	FiveHour float64
	SevenDay float64
	Stale    string // "15m", "", or "reset"
}

// SectionVisibility controls which statusline sections are rendered.
// All fields default to true when nil.
type SectionVisibility struct {
	ShowAccount      *bool
	ShowCost         *bool
	ShowContext      *bool
	ShowRate5h       *bool
	ShowRate7d       *bool
	ShowOtherAccount *bool
	ShowSwitchHint   *bool
}

// isEnabled returns the value of a *bool field, defaulting to true when nil.
func isEnabled(field *bool) bool {
	if field == nil {
		return true
	}
	return *field
}

// RenderOpts holds optional rendering parameters.
type RenderOpts struct {
	AccountName string             // current account label (empty = omit)
	Compact     bool               // compact single-line mode
	Sections    *SectionVisibility // nil = show all sections
}

// Render produces one or two status-line strings.
//
// Line 1: [20x] [Opus 4.6] 32% ctx | $0.45 | 5h: ██░░░ 38% (2h14m) | 7d: ███░░ 62%
// Line 1 may end with: → alt (yellow, when current 5h > 80% and alt has lower usage)
// Line 2 (if other != nil): [5x] 5h: ████░ 71% | stale 15m
//
// Compact mode: [20x] [Opus] 32% | 5h:38% 7d:62%
//
// When no rate_limits: [Opus 4.6] 32% ctx
// When context used_percentage is nil: treat as 0.
func Render(input *Input, other *OtherAccount, useColor bool, opts ...RenderOpts) []string {
	var opt RenderOpts
	if len(opts) > 0 {
		opt = opts[0]
	}

	if opt.Compact {
		return []string{renderCompact(input, opt, useColor)}
	}

	var lines []string
	lines = append(lines, renderPrimary(input, other, opt, useColor))
	showOther := opt.Sections == nil || isEnabled(opt.Sections.ShowOtherAccount)
	if other != nil && showOther {
		lines = append(lines, renderOther(other, useColor))
	}
	return lines
}

func renderCompact(input *Input, opt RenderOpts, useColor bool) string {
	sec := opt.Sections

	ctxPct := 0.0
	if input.ContextWindow.UsedPercentage != nil {
		ctxPct = *input.ContextWindow.UsedPercentage
	}

	var parts []string

	// Account label.
	showAccount := sec == nil || isEnabled(sec.ShowAccount)
	if opt.AccountName != "" && showAccount {
		nameStr := format.Colorize(opt.AccountName, format.Cyan, useColor)
		parts = append(parts, fmt.Sprintf("[%s]", nameStr))
	}

	// Shortened model name (first word only) + context.
	shortModel := input.Model.DisplayName
	if idx := strings.Index(shortModel, " "); idx > 0 {
		shortModel = shortModel[:idx]
	}
	modelStr := format.Colorize(shortModel, format.Cyan+format.Bold, useColor)
	showContext := sec == nil || isEnabled(sec.ShowContext)
	if showContext {
		ctxStr := format.Colorize(fmt.Sprintf("%d%%", int(ctxPct)), format.UsageColor(ctxPct), useColor)
		parts = append(parts, fmt.Sprintf("[%s] %s", modelStr, ctxStr))
	} else {
		parts = append(parts, fmt.Sprintf("[%s]", modelStr))
	}

	// Rate limits as compact percentages.
	if input.RateLimits != nil {
		show5h := sec == nil || isEnabled(sec.ShowRate5h)
		if input.RateLimits.FiveHour != nil && show5h {
			pct := int(math.Round(input.RateLimits.FiveHour.UsedPercentage))
			pctStr := format.Colorize(fmt.Sprintf("%d%%", pct), format.UsageColor(input.RateLimits.FiveHour.UsedPercentage), useColor)
			parts = append(parts, fmt.Sprintf("5h:%s", pctStr))
		}
		show7d := sec == nil || isEnabled(sec.ShowRate7d)
		if input.RateLimits.SevenDay != nil && show7d {
			pct := int(math.Round(input.RateLimits.SevenDay.UsedPercentage))
			pctStr := format.Colorize(fmt.Sprintf("%d%%", pct), format.UsageColor(input.RateLimits.SevenDay.UsedPercentage), useColor)
			parts = append(parts, fmt.Sprintf("7d:%s", pctStr))
		}
	}

	return strings.Join(parts, " | ")
}

func renderPrimary(input *Input, other *OtherAccount, opt RenderOpts, useColor bool) string {
	sec := opt.Sections

	ctxPct := 0.0
	if input.ContextWindow.UsedPercentage != nil {
		ctxPct = *input.ContextWindow.UsedPercentage
	}

	var prefix string
	showAccount := sec == nil || isEnabled(sec.ShowAccount)
	if opt.AccountName != "" && showAccount {
		nameStr := format.Colorize(opt.AccountName, format.Cyan, useColor)
		prefix = fmt.Sprintf("[%s] ", nameStr)
	}

	modelName := format.Colorize(input.Model.DisplayName, format.Cyan+format.Bold, useColor)

	showContext := sec == nil || isEnabled(sec.ShowContext)
	if showContext {
		ctxStr := format.Colorize(fmt.Sprintf("%d%%", int(ctxPct)), format.UsageColor(ctxPct), useColor)
		prefix += fmt.Sprintf("[%s] %s ctx", modelName, ctxStr)
	} else {
		prefix += fmt.Sprintf("[%s]", modelName)
	}
	line := prefix

	// Session cost.
	showCost := sec == nil || isEnabled(sec.ShowCost)
	if showCost && input.Cost != nil && input.Cost.TotalCostUSD != nil {
		cost := *input.Cost.TotalCostUSD
		costStr := format.Colorize(fmt.Sprintf("$%.2f", cost), format.Dim, useColor)
		line += fmt.Sprintf(" | %s", costStr)
	}

	if input.RateLimits == nil {
		return line
	}

	show5h := sec == nil || isEnabled(sec.ShowRate5h)
	if input.RateLimits.FiveHour != nil && show5h {
		w := input.RateLimits.FiveHour
		bar := colorProgressBar(w.UsedPercentage, barWidth, useColor)
		pctStr := format.Colorize(fmt.Sprintf("%d%%", int(math.Round(w.UsedPercentage))), format.UsageColor(w.UsedPercentage), useColor)
		ttl := format.FormatDuration(time.Until(time.Unix(w.ResetsAt, 0)))
		line += fmt.Sprintf(" | 5h: %s %s (%s)", bar, pctStr, ttl)
	}

	show7d := sec == nil || isEnabled(sec.ShowRate7d)
	if input.RateLimits.SevenDay != nil && show7d {
		w := input.RateLimits.SevenDay
		bar := colorProgressBar(w.UsedPercentage, barWidth, useColor)
		pctStr := format.Colorize(fmt.Sprintf("%d%%", int(math.Round(w.UsedPercentage))), format.UsageColor(w.UsedPercentage), useColor)
		line += fmt.Sprintf(" | 7d: %s %s", bar, pctStr)
	}

	// Smart switch prompt: when current 5h > 80% and another account has lower usage.
	showHint := sec == nil || isEnabled(sec.ShowSwitchHint)
	if showHint {
		line += renderSwitchHint(input, other, useColor)
	}

	return line
}

// renderSwitchHint appends a switch suggestion when the current account's 5h
// usage exceeds 80% and the other account has lower 5h usage.
func renderSwitchHint(input *Input, other *OtherAccount, useColor bool) string {
	if other == nil || input.RateLimits == nil || input.RateLimits.FiveHour == nil {
		return ""
	}
	currentUsage := input.RateLimits.FiveHour.UsedPercentage
	if currentUsage <= 80 {
		return ""
	}
	if other.FiveHour >= currentUsage {
		return ""
	}
	arrow := format.Colorize(fmt.Sprintf(" -> %s", other.Name), format.Yellow, useColor)
	return arrow
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
