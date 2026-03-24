package format

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// ProgressBar renders a filled/empty bar of the given width for pct ∈ [0,100].
// Example for pct=50, width=10: "█████░░░░░"
func ProgressBar(pct float64, width int) string {
	if width <= 0 {
		return ""
	}
	pct = math.Max(0, math.Min(100, pct))
	filled := int(math.Round(pct / 100.0 * float64(width)))
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// FormatDuration formats d as a compact human-readable string.
// Examples: 2h14m, 48m, 0m.
func FormatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
