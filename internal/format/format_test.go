package format

import (
	"testing"
	"time"
)

func TestProgressBar(t *testing.T) {
	tests := []struct {
		pct   float64
		width int
		want  string
	}{
		{0, 5, "░░░░░"},
		{100, 5, "█████"},
		{50, 4, "██░░"},
		{20, 5, "█░░░░"},
	}
	for _, tt := range tests {
		got := ProgressBar(tt.pct, tt.width)
		if got != tt.want {
			t.Errorf("ProgressBar(%v, %d) = %q, want %q", tt.pct, tt.width, got, tt.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input string // parseable duration
		want  string
	}{
		{"30s", "0m"},
		{"5m", "5m"},
		{"90m", "1h30m"},
		{"3h", "3h0m"},
		{"25h", "25h0m"},
	}
	for _, tt := range tests {
		d, _ := time.ParseDuration(tt.input)
		got := FormatDuration(d)
		if got != tt.want {
			t.Errorf("FormatDuration(%s) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{1234567, "1,234,567"},
	}
	for _, tt := range tests {
		got := FormatNumber(tt.n)
		if got != tt.want {
			t.Errorf("FormatNumber(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestUsageColor(t *testing.T) {
	if UsageColor(30) != Green {
		t.Error("30% should be green")
	}
	if UsageColor(60) != Yellow {
		t.Error("60% should be yellow")
	}
	if UsageColor(90) != Red {
		t.Error("90% should be red")
	}
}

func TestColorize_Disabled(t *testing.T) {
	got := Colorize("hello", Red, false)
	if got != "hello" {
		t.Errorf("Colorize with disabled should return raw text, got %q", got)
	}
}

func TestColorize_Enabled(t *testing.T) {
	got := Colorize("hello", Red, true)
	if got == "hello" {
		t.Error("Colorize with enabled should wrap in ANSI codes")
	}
	if len(got) <= len("hello") {
		t.Error("Colorize output should be longer than input (ANSI codes)")
	}
}

func TestIsEnabled(t *testing.T) {
	tr := true
	fa := false
	if !IsEnabled(&tr) {
		t.Error("IsEnabled(true) should return true")
	}
	if IsEnabled(&fa) {
		t.Error("IsEnabled(false) should return false")
	}
	if !IsEnabled(nil) {
		t.Error("IsEnabled(nil) should default to true")
	}
}
