package statusline

import (
	"fmt"
	"math"
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
	lines = append(lines, renderPrimary(input))
	if other != nil {
		lines = append(lines, renderOther(other))
	}
	return lines
}

func renderPrimary(input *Input) string {
	ctxPct := 0.0
	if input.ContextWindow.UsedPercentage != nil {
		ctxPct = *input.ContextWindow.UsedPercentage
	}

	line := fmt.Sprintf("[%s] %d%% ctx", input.Model.DisplayName, int(ctxPct))

	if input.RateLimits == nil {
		return line
	}

	if input.RateLimits.FiveHour != nil {
		w := input.RateLimits.FiveHour
		bar := format.ProgressBar(w.UsedPercentage, barWidth)
		ttl := format.FormatDuration(time.Until(time.Unix(w.ResetsAt, 0)))
		line += fmt.Sprintf(" | 5h: %s %d%% (%s)", bar, int(math.Round(w.UsedPercentage)), ttl)
	}

	if input.RateLimits.SevenDay != nil {
		w := input.RateLimits.SevenDay
		bar := format.ProgressBar(w.UsedPercentage, barWidth)
		line += fmt.Sprintf(" | 7d: %s %d%%", bar, int(math.Round(w.UsedPercentage)))
	}

	return line
}

func renderOther(other *OtherAccount) string {
	bar5h := format.ProgressBar(other.FiveHour, barWidth)
	line := fmt.Sprintf("[%s] 5h: %s %d%%", other.Name, bar5h, int(math.Round(other.FiveHour)))

	if other.SevenDay > 0 {
		bar7d := format.ProgressBar(other.SevenDay, barWidth)
		line += fmt.Sprintf(" | 7d: %s %d%%", bar7d, int(math.Round(other.SevenDay)))
	}

	if other.Stale != "" {
		if other.Stale == "reset" {
			line += " | reset"
		} else {
			line += fmt.Sprintf(" | stale %s", other.Stale)
		}
	}

	return line
}
