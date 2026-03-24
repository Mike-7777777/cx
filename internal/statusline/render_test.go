package statusline

import (
	"strings"
	"testing"
)

func ptrF64(v float64) *float64 { return &v }
func ptrI64(v int64) *int64     { return &v }

func TestRender_Full(t *testing.T) {
	input := &Input{
		Model:         Model{ID: "claude-opus-4-6", DisplayName: "Opus 4.6"},
		ContextWindow: ContextWindow{UsedPercentage: ptrF64(32.5), ContextWindowSize: ptrI64(200000)},
		RateLimits: &InputRateLimits{
			FiveHour: &RateWindow{UsedPercentage: 38.2, ResetsAt: 9999999999},
			SevenDay: &RateWindow{UsedPercentage: 62.0, ResetsAt: 9999999999},
		},
	}

	lines := Render(input, nil, false)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}

	line := lines[0]
	// Must contain model name
	if !strings.Contains(line, "Opus 4.6") {
		t.Errorf("line missing model name: %q", line)
	}
	// Must contain context percentage
	if !strings.Contains(line, "32%") {
		t.Errorf("line missing context%%: %q", line)
	}
	// Must contain rate limit labels
	if !strings.Contains(line, "5h:") {
		t.Errorf("line missing 5h rate limit: %q", line)
	}
	if !strings.Contains(line, "7d:") {
		t.Errorf("line missing 7d rate limit: %q", line)
	}
	// Must contain percentage values
	if !strings.Contains(line, "38%") {
		t.Errorf("line missing 5h percentage: %q", line)
	}
	if !strings.Contains(line, "62%") {
		t.Errorf("line missing 7d percentage: %q", line)
	}
	// Must contain progress bar characters
	if !strings.Contains(line, "█") {
		t.Errorf("line missing filled bar char: %q", line)
	}
	if !strings.Contains(line, "░") {
		t.Errorf("line missing empty bar char: %q", line)
	}
}

func TestRender_Minimal(t *testing.T) {
	input := &Input{
		Model:         Model{ID: "claude-opus-4-6", DisplayName: "Opus 4.6"},
		ContextWindow: ContextWindow{UsedPercentage: ptrF64(15.0)},
	}

	lines := Render(input, nil, false)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}

	line := lines[0]
	if !strings.Contains(line, "Opus 4.6") {
		t.Errorf("line missing model name: %q", line)
	}
	if !strings.Contains(line, "15%") {
		t.Errorf("line missing context%%: %q", line)
	}
	// No rate limit info when absent
	if strings.Contains(line, "5h:") {
		t.Errorf("line should not contain rate limits: %q", line)
	}
}

func TestRender_NilContext(t *testing.T) {
	input := &Input{
		Model:         Model{ID: "claude-opus-4-6", DisplayName: "Opus 4.6"},
		ContextWindow: ContextWindow{}, // nil UsedPercentage
	}

	lines := Render(input, nil, false)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}

	// Nil context treated as 0%
	if !strings.Contains(lines[0], "0%") {
		t.Errorf("nil context should render as 0%%: %q", lines[0])
	}
}

func TestRender_WithOtherAccount(t *testing.T) {
	input := &Input{
		Model:         Model{ID: "claude-opus-4-6", DisplayName: "Opus 4.6"},
		ContextWindow: ContextWindow{UsedPercentage: ptrF64(10.0)},
		RateLimits: &InputRateLimits{
			FiveHour: &RateWindow{UsedPercentage: 20.0, ResetsAt: 9999999999},
		},
	}

	other := &OtherAccount{
		Name:     "5x",
		FiveHour: 71.0,
		SevenDay: 50.0,
		Stale:    "15m",
	}

	lines := Render(input, other, false)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}

	line2 := lines[1]
	if !strings.Contains(line2, "5x") {
		t.Errorf("line2 missing account name: %q", line2)
	}
	if !strings.Contains(line2, "71%") {
		t.Errorf("line2 missing 5h percentage: %q", line2)
	}
	if !strings.Contains(line2, "stale 15m") {
		t.Errorf("line2 missing stale info: %q", line2)
	}
}

func TestRender_OtherAccountReset(t *testing.T) {
	input := &Input{
		Model:         Model{ID: "claude-opus-4-6", DisplayName: "Opus 4.6"},
		ContextWindow: ContextWindow{UsedPercentage: ptrF64(10.0)},
	}

	other := &OtherAccount{
		Name:     "alt",
		FiveHour: 80.0,
		SevenDay: 30.0,
		Stale:    "reset",
	}

	lines := Render(input, other, false)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}

	if !strings.Contains(lines[1], "reset") {
		t.Errorf("line2 missing reset marker: %q", lines[1])
	}
}
