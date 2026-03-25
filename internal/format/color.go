package format

// ANSI color codes.
const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Red       = "\033[31m"
	Green     = "\033[32m"
	Yellow    = "\033[33m"
	Blue      = "\033[34m"
	Magenta   = "\033[35m"
	Cyan      = "\033[36m"
	White     = "\033[37m"
	BrightRed = "\033[91m"
)

// Colorize wraps text in ANSI color codes. Returns plain text if color is disabled.
func Colorize(text, color string, enabled bool) string {
	if !enabled {
		return text
	}
	return color + text + Reset
}

// UsageColor returns the ANSI color code appropriate for the given percentage:
// Green (<50%), Yellow (50–80%), Red (>80%).
func UsageColor(pct float64) string {
	switch {
	case pct > 80:
		return Red
	case pct >= 50:
		return Yellow
	default:
		return Green
	}
}
