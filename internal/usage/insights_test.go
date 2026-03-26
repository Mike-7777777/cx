package usage

import (
	"math"
	"testing"
	"time"
)

// makeEntry is a convenience constructor for test entries.
func makeEntry(model string, hour int, input, output, cacheCreate, cacheRead int64) Entry {
	return Entry{
		Model: model,
		Usage: TokenUsage{
			InputTokens:              input,
			OutputTokens:             output,
			CacheCreationInputTokens: cacheCreate,
			CacheReadInputTokens:     cacheRead,
		},
		Timestamp: time.Date(2026, 3, 24, hour, 0, 0, 0, time.UTC),
	}
}

// TestAggregateHourly_GroupsByHour verifies that entries at different hours
// are placed into the correct HourlyReport buckets.
func TestAggregateHourly_GroupsByHour(t *testing.T) {
	entries := []Entry{
		makeEntry("claude-opus-4-6", 9, 100, 200, 0, 0),
		makeEntry("claude-opus-4-6", 9, 50, 100, 0, 0),
		makeEntry("claude-sonnet-4-6", 14, 80, 150, 0, 0),
	}

	reports := AggregateHourly(entries)

	if len(reports) != 2 {
		t.Fatalf("expected 2 hourly reports, got %d", len(reports))
	}

	// After ascending sort: index 0 = hour 9, index 1 = hour 14.
	h9 := reports[0]
	if h9.Hour != 9 {
		t.Errorf("first report hour: got %d, want 9", h9.Hour)
	}
	if h9.Summary.InputTokens != 150 { // 100 + 50
		t.Errorf("hour 9 input: got %d, want 150", h9.Summary.InputTokens)
	}
	if h9.Summary.OutputTokens != 300 { // 200 + 100
		t.Errorf("hour 9 output: got %d, want 300", h9.Summary.OutputTokens)
	}
	if h9.Summary.EntryCount != 2 {
		t.Errorf("hour 9 entry count: got %d, want 2", h9.Summary.EntryCount)
	}

	h14 := reports[1]
	if h14.Hour != 14 {
		t.Errorf("second report hour: got %d, want 14", h14.Hour)
	}
	if h14.Summary.InputTokens != 80 {
		t.Errorf("hour 14 input: got %d, want 80", h14.Summary.InputTokens)
	}
	if h14.Summary.EntryCount != 1 {
		t.Errorf("hour 14 entry count: got %d, want 1", h14.Summary.EntryCount)
	}
}

// TestAggregateHourly_SortedByHour verifies that the result is in ascending
// hour order regardless of the input order.
func TestAggregateHourly_SortedByHour(t *testing.T) {
	// Feed entries in reverse hour order.
	entries := []Entry{
		makeEntry("claude-opus-4-6", 23, 10, 20, 0, 0),
		makeEntry("claude-opus-4-6", 5, 30, 40, 0, 0),
		makeEntry("claude-opus-4-6", 12, 50, 60, 0, 0),
		makeEntry("claude-opus-4-6", 0, 70, 80, 0, 0),
	}

	reports := AggregateHourly(entries)

	if len(reports) != 4 {
		t.Fatalf("expected 4 hourly reports, got %d", len(reports))
	}

	expectedHours := []int{0, 5, 12, 23}
	for i, want := range expectedHours {
		if reports[i].Hour != want {
			t.Errorf("reports[%d].Hour: got %d, want %d", i, reports[i].Hour, want)
		}
	}
}

// TestAggregateModelDistribution_CalculatesPercentage verifies that percentage
// fields are computed and that all cost percentages sum to approximately 100.
func TestAggregateModelDistribution_CalculatesPercentage(t *testing.T) {
	entries := []Entry{
		makeEntry("claude-opus-4-6", 10, 1000, 2000, 0, 0),
		makeEntry("claude-opus-4-6", 11, 500, 1000, 0, 0),
		makeEntry("claude-sonnet-4-6", 12, 2000, 3000, 0, 0),
	}

	reports := AggregateModelDistribution(entries)

	if len(reports) != 2 {
		t.Fatalf("expected 2 model reports, got %d", len(reports))
	}

	// Percentages must sum to ~100.
	var totalCostPct, totalTokenPct, totalMsgPct float64
	for _, r := range reports {
		totalCostPct += r.CostPercent
		totalTokenPct += r.TokenPercent
		totalMsgPct += r.MsgPercent
	}

	const epsilon = 0.001
	if math.Abs(totalCostPct-100.0) > epsilon {
		t.Errorf("cost percentages sum: got %.4f, want ~100", totalCostPct)
	}
	if math.Abs(totalTokenPct-100.0) > epsilon {
		t.Errorf("token percentages sum: got %.4f, want ~100", totalTokenPct)
	}
	if math.Abs(totalMsgPct-100.0) > epsilon {
		t.Errorf("msg percentages sum: got %.4f, want ~100", totalMsgPct)
	}

	// Results must be sorted by cost descending (opus is more expensive per token).
	if reports[0].Model != "claude-opus-4-6" {
		t.Errorf("first model: got %q, want claude-opus-4-6", reports[0].Model)
	}

	// All individual percentages must be in (0, 100].
	for _, r := range reports {
		if r.CostPercent <= 0 || r.CostPercent > 100 {
			t.Errorf("model %q CostPercent out of range: %.4f", r.Model, r.CostPercent)
		}
	}
}

// TestCalculateEfficiency_CacheHitRatio verifies that the cache hit ratio is
// cache_reads / (cache_reads + cache_creates).
func TestCalculateEfficiency_CacheHitRatio(t *testing.T) {
	entries := []Entry{
		// cache_reads=300, cache_creates=100 → ratio = 300/400 = 0.75
		makeEntry("claude-opus-4-6", 10, 100, 200, 100, 300),
	}

	m := CalculateEfficiency(entries)

	const wantRatio = 0.75
	if math.Abs(m.CacheHitRatio-wantRatio) > 0.0001 {
		t.Errorf("CacheHitRatio: got %.4f, want %.4f", m.CacheHitRatio, wantRatio)
	}

	// AvgTokensPerMsg: total = 100+200+100+300 = 700; 1 message → 700
	if m.AvgTokensPerMsg != 700 {
		t.Errorf("AvgTokensPerMsg: got %d, want 700", m.AvgTokensPerMsg)
	}

	if m.AvgCostPerMsg <= 0 {
		t.Error("AvgCostPerMsg should be positive")
	}

	// InputOutputRatio = 100 / 200 = 0.5
	const wantIORatio = 0.5
	if math.Abs(m.InputOutputRatio-wantIORatio) > 0.0001 {
		t.Errorf("InputOutputRatio: got %.4f, want %.4f", m.InputOutputRatio, wantIORatio)
	}
}

// TestCalculateEfficiency_ZeroCacheTokens verifies that CacheHitRatio is zero
// when there are no cache tokens.
func TestCalculateEfficiency_ZeroCacheTokens(t *testing.T) {
	entries := []Entry{
		makeEntry("claude-opus-4-6", 10, 100, 200, 0, 0),
	}

	m := CalculateEfficiency(entries)

	if m.CacheHitRatio != 0 {
		t.Errorf("CacheHitRatio with no cache tokens: got %.4f, want 0", m.CacheHitRatio)
	}
}

// TestFindPeakHours_TopN verifies that FindPeakHours returns the top N hours
// ordered by total token count descending.
func TestFindPeakHours_TopN(t *testing.T) {
	entries := []Entry{
		// hour 10: 300 tokens total
		makeEntry("claude-opus-4-6", 10, 100, 200, 0, 0),
		// hour 14: 2300 tokens total (clear winner)
		makeEntry("claude-opus-4-6", 14, 500, 1000, 300, 500),
		// hour 22: 50 tokens total
		makeEntry("claude-sonnet-4-6", 22, 20, 30, 0, 0),
	}

	top2 := FindPeakHours(entries, 2)

	if len(top2) != 2 {
		t.Fatalf("expected 2 peak hours, got %d", len(top2))
	}

	// Hour 14 has the most tokens and must be first.
	if top2[0].Hour != 14 {
		t.Errorf("peak hour 1: got %d, want 14", top2[0].Hour)
	}
	if top2[1].Hour != 10 {
		t.Errorf("peak hour 2: got %d, want 10", top2[1].Hour)
	}

	// Verify descending order.
	if top2[0].Summary.TotalTokens < top2[1].Summary.TotalTokens {
		t.Errorf("results not in descending token order: %d < %d",
			top2[0].Summary.TotalTokens, top2[1].Summary.TotalTokens)
	}
}

// TestFindPeakHours_NGreaterThanAvailable verifies that requesting more hours
// than exist returns all available hours without panic.
func TestFindPeakHours_NGreaterThanAvailable(t *testing.T) {
	entries := []Entry{
		makeEntry("claude-opus-4-6", 9, 100, 200, 0, 0),
	}

	result := FindPeakHours(entries, 10)
	if len(result) != 1 {
		t.Errorf("expected 1 result (capped), got %d", len(result))
	}
}
