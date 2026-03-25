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

// FormatNumber formats an int64 with comma separators (e.g., 1234567 -> "1,234,567").
func FormatNumber(n int64) string {
	if n < 0 {
		return "-" + FormatNumber(-n)
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var buf strings.Builder
	remainder := len(s) % 3
	if remainder > 0 {
		buf.WriteString(s[:remainder])
	}
	for i := remainder; i < len(s); i += 3 {
		if buf.Len() > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(s[i : i+3])
	}
	return buf.String()
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

// IsEnabled returns the value of a *bool field, defaulting to true when nil.
// This is the canonical implementation used across config and statusline packages.
func IsEnabled(field *bool) bool {
	if field == nil {
		return true
	}
	return *field
}
