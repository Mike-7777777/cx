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

func TestRender_ColorEnabled(t *testing.T) {
	input := &Input{
		Model:         Model{ID: "claude-opus-4-6", DisplayName: "Opus 4.6"},
		ContextWindow: ContextWindow{UsedPercentage: ptrF64(32.5), ContextWindowSize: ptrI64(200000)},
		RateLimits: &InputRateLimits{
			FiveHour: &RateWindow{UsedPercentage: 38.2, ResetsAt: 9999999999},
			SevenDay: &RateWindow{UsedPercentage: 85.0, ResetsAt: 9999999999},
		},
	}

	lines := Render(input, nil, true)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}

	line := lines[0]
	// ANSI escape sequences must be present when color is enabled.
	if !strings.Contains(line, "\033[") {
		t.Errorf("colored line missing ANSI escape codes: %q", line)
	}
	// Content must still be present (color codes wrap the text, not replace it).
	if !strings.Contains(line, "Opus 4.6") {
		t.Errorf("colored line missing model name: %q", line)
	}
	if !strings.Contains(line, "38%") {
		t.Errorf("colored line missing 5h percentage: %q", line)
	}
}

func TestRender_AccountLabel(t *testing.T) {
	input := &Input{
		Model:         Model{ID: "claude-opus-4-6", DisplayName: "Opus 4.6"},
		ContextWindow: ContextWindow{UsedPercentage: ptrF64(32.5)},
		RateLimits: &InputRateLimits{
			FiveHour: &RateWindow{UsedPercentage: 38.2, ResetsAt: 9999999999},
		},
	}

	opts := RenderOpts{AccountName: "20x"}
	lines := Render(input, nil, false, opts)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}

	line := lines[0]
	if !strings.Contains(line, "[20x]") {
		t.Errorf("line missing account label: %q", line)
	}
	if !strings.Contains(line, "Opus 4.6") {
		t.Errorf("line missing model name: %q", line)
	}
}

func TestRender_AccountLabelEmpty(t *testing.T) {
	input := &Input{
		Model:         Model{ID: "claude-opus-4-6", DisplayName: "Opus 4.6"},
		ContextWindow: ContextWindow{UsedPercentage: ptrF64(15.0)},
	}

	// No account name: should not have extra brackets at start.
	opts := RenderOpts{AccountName: ""}
	lines := Render(input, nil, false, opts)
	line := lines[0]
	if strings.HasPrefix(line, "[] ") {
		t.Errorf("empty account name should not produce empty brackets: %q", line)
	}
}

func TestRender_SessionCost(t *testing.T) {
	input := &Input{
		Model:         Model{ID: "claude-opus-4-6", DisplayName: "Opus 4.6"},
		ContextWindow: ContextWindow{UsedPercentage: ptrF64(32.5)},
		Cost:          &Cost{TotalCostUSD: ptrF64(0.45)},
		RateLimits: &InputRateLimits{
			FiveHour: &RateWindow{UsedPercentage: 38.2, ResetsAt: 9999999999},
		},
	}

	lines := Render(input, nil, false)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}

	line := lines[0]
	if !strings.Contains(line, "$0.45") {
		t.Errorf("line missing session cost: %q", line)
	}
}

func TestRender_SessionCostNil(t *testing.T) {
	input := &Input{
		Model:         Model{ID: "claude-opus-4-6", DisplayName: "Opus 4.6"},
		ContextWindow: ContextWindow{UsedPercentage: ptrF64(15.0)},
	}

	lines := Render(input, nil, false)
	line := lines[0]
	if strings.Contains(line, "$") {
		t.Errorf("nil cost should not show dollar sign: %q", line)
	}
}

func TestRender_SmartSwitchHint(t *testing.T) {
	input := &Input{
		Model:         Model{ID: "claude-opus-4-6", DisplayName: "Opus 4.6"},
		ContextWindow: ContextWindow{UsedPercentage: ptrF64(10.0)},
		RateLimits: &InputRateLimits{
			FiveHour: &RateWindow{UsedPercentage: 85.0, ResetsAt: 9999999999},
		},
	}

	other := &OtherAccount{
		Name:     "alt",
		FiveHour: 30.0,
	}

	lines := Render(input, other, false)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}

	// Line 1 should contain switch hint.
	if !strings.Contains(lines[0], "-> alt") {
		t.Errorf("line1 missing switch hint: %q", lines[0])
	}
}

func TestRender_SmartSwitchNoHintBelow80(t *testing.T) {
	input := &Input{
		Model:         Model{ID: "claude-opus-4-6", DisplayName: "Opus 4.6"},
		ContextWindow: ContextWindow{UsedPercentage: ptrF64(10.0)},
		RateLimits: &InputRateLimits{
			FiveHour: &RateWindow{UsedPercentage: 75.0, ResetsAt: 9999999999},
		},
	}

	other := &OtherAccount{
		Name:     "alt",
		FiveHour: 30.0,
	}

	lines := Render(input, other, false)
	// Should NOT contain switch hint when < 80%.
	if strings.Contains(lines[0], "-> alt") {
		t.Errorf("line1 should not have switch hint below 80%%: %q", lines[0])
	}
}

func TestRender_SmartSwitchNoHintOtherHigher(t *testing.T) {
	input := &Input{
		Model:         Model{ID: "claude-opus-4-6", DisplayName: "Opus 4.6"},
		ContextWindow: ContextWindow{UsedPercentage: ptrF64(10.0)},
		RateLimits: &InputRateLimits{
			FiveHour: &RateWindow{UsedPercentage: 90.0, ResetsAt: 9999999999},
		},
	}

	other := &OtherAccount{
		Name:     "alt",
		FiveHour: 95.0,
	}

	lines := Render(input, other, false)
	// Should NOT contain switch hint when other has higher usage.
	if strings.Contains(lines[0], "-> alt") {
		t.Errorf("line1 should not suggest switching to worse account: %q", lines[0])
	}
}

func TestRender_Compact(t *testing.T) {
	input := &Input{
		Model:         Model{ID: "claude-opus-4-6", DisplayName: "Opus 4.6"},
		ContextWindow: ContextWindow{UsedPercentage: ptrF64(32.5)},
		RateLimits: &InputRateLimits{
			FiveHour: &RateWindow{UsedPercentage: 38.0, ResetsAt: 9999999999},
			SevenDay: &RateWindow{UsedPercentage: 62.0, ResetsAt: 9999999999},
		},
	}

	opts := RenderOpts{AccountName: "20x", Compact: true}
	lines := Render(input, nil, false, opts)

	if len(lines) != 1 {
		t.Fatalf("compact mode should produce 1 line, got %d", len(lines))
	}

	line := lines[0]
	if !strings.Contains(line, "[20x]") {
		t.Errorf("compact line missing account label: %q", line)
	}
	// Compact uses shortened model name (first word only).
	if !strings.Contains(line, "Opus") {
		t.Errorf("compact line missing model name: %q", line)
	}
	if !strings.Contains(line, "5h:") {
		t.Errorf("compact line missing 5h: %q", line)
	}
	if !strings.Contains(line, "7d:") {
		t.Errorf("compact line missing 7d: %q", line)
	}
	// Should NOT contain progress bars.
	if strings.Contains(line, "█") || strings.Contains(line, "░") {
		t.Errorf("compact line should not contain progress bars: %q", line)
	}
}

func TestRender_CompactNoOtherLine(t *testing.T) {
	input := &Input{
		Model:         Model{ID: "claude-opus-4-6", DisplayName: "Opus 4.6"},
		ContextWindow: ContextWindow{UsedPercentage: ptrF64(10.0)},
		RateLimits: &InputRateLimits{
			FiveHour: &RateWindow{UsedPercentage: 85.0, ResetsAt: 9999999999},
		},
	}

	other := &OtherAccount{
		Name:     "alt",
		FiveHour: 30.0,
	}

	opts := RenderOpts{Compact: true}
	lines := Render(input, other, false, opts)

	// Compact mode always outputs exactly 1 line, even with other account.
	if len(lines) != 1 {
		t.Fatalf("compact mode should produce 1 line even with other account, got %d", len(lines))
	}
}

func TestRender_FullWithAllFeatures(t *testing.T) {
	input := &Input{
		Model:         Model{ID: "claude-opus-4-6", DisplayName: "Opus 4.6"},
		ContextWindow: ContextWindow{UsedPercentage: ptrF64(32.5), ContextWindowSize: ptrI64(200000)},
		RateLimits: &InputRateLimits{
			FiveHour: &RateWindow{UsedPercentage: 85.0, ResetsAt: 9999999999},
			SevenDay: &RateWindow{UsedPercentage: 62.0, ResetsAt: 9999999999},
		},
		Cost: &Cost{TotalCostUSD: ptrF64(1.23)},
	}

	other := &OtherAccount{
		Name:     "5x",
		FiveHour: 30.0,
		SevenDay: 20.0,
	}

	opts := RenderOpts{AccountName: "20x"}
	lines := Render(input, other, false, opts)

	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}

	line1 := lines[0]
	// Account label
	if !strings.Contains(line1, "[20x]") {
		t.Errorf("line1 missing account label: %q", line1)
	}
	// Model
	if !strings.Contains(line1, "Opus 4.6") {
		t.Errorf("line1 missing model: %q", line1)
	}
	// Cost
	if !strings.Contains(line1, "$1.23") {
		t.Errorf("line1 missing cost: %q", line1)
	}
	// Smart switch hint (85% > 80%, other at 30%)
	if !strings.Contains(line1, "-> 5x") {
		t.Errorf("line1 missing switch hint: %q", line1)
	}
}
