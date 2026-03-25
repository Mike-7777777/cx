package usage

import (
	"encoding/json"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// loadSampleEntries parses the testdata sample file and returns all entries.
func loadSampleEntries(t *testing.T) []Entry {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "usage_sample.jsonl")
	var entries []Entry
	if err := ParseFile(path, func(e Entry) {
		entries = append(entries, e)
	}); err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	return entries
}

func TestAggregateDailies(t *testing.T) {
	entries := loadSampleEntries(t)
	reports := AggregateDailies(entries)

	if len(reports) != 2 {
		t.Fatalf("expected 2 daily reports, got %d", len(reports))
	}

	// Day 1: 2026-03-24 (entry 0 + entry 1)
	d1 := reports[0]
	if d1.Date != "2026-03-24" {
		t.Errorf("day 1 date: got %q, want %q", d1.Date, "2026-03-24")
	}
	if d1.Summary.InputTokens != 150 { // 100 + 50
		t.Errorf("day 1 input: got %d, want 150", d1.Summary.InputTokens)
	}
	if d1.Summary.OutputTokens != 500 { // 200 + 300
		t.Errorf("day 1 output: got %d, want 500", d1.Summary.OutputTokens)
	}
	if d1.Summary.CacheCreationInputTokens != 500 { // 500 + 0
		t.Errorf("day 1 cache_create: got %d, want 500", d1.Summary.CacheCreationInputTokens)
	}
	if d1.Summary.CacheReadInputTokens != 3000 { // 1000 + 2000
		t.Errorf("day 1 cache_read: got %d, want 3000", d1.Summary.CacheReadInputTokens)
	}
	if d1.Summary.TotalTokens != 4150 { // 150+500+500+3000
		t.Errorf("day 1 total: got %d, want 4150", d1.Summary.TotalTokens)
	}
	if d1.Summary.EntryCount != 2 {
		t.Errorf("day 1 entry count: got %d, want 2", d1.Summary.EntryCount)
	}
	// Cost = entry0 cost + entry1 cost = 0.027375 + 0.00525 = 0.032625
	if math.Abs(d1.Summary.CostUSD-0.032625) > 0.0001 {
		t.Errorf("day 1 cost: got %.6f, want 0.032625", d1.Summary.CostUSD)
	}

	// Verify per-model breakdown for day 1
	if len(d1.Models) != 2 {
		t.Fatalf("day 1 models: expected 2, got %d", len(d1.Models))
	}
	opusDay1 := d1.Models["claude-opus-4-6"]
	if opusDay1.InputTokens != 100 || opusDay1.OutputTokens != 200 {
		t.Errorf("day 1 opus tokens: input=%d output=%d", opusDay1.InputTokens, opusDay1.OutputTokens)
	}
	sonnetDay1 := d1.Models["claude-sonnet-4-6"]
	if sonnetDay1.InputTokens != 50 || sonnetDay1.OutputTokens != 300 {
		t.Errorf("day 1 sonnet tokens: input=%d output=%d", sonnetDay1.InputTokens, sonnetDay1.OutputTokens)
	}

	// Day 2: 2026-03-25 (entry 2 only)
	d2 := reports[1]
	if d2.Date != "2026-03-25" {
		t.Errorf("day 2 date: got %q, want %q", d2.Date, "2026-03-25")
	}
	if d2.Summary.InputTokens != 80 {
		t.Errorf("day 2 input: got %d, want 80", d2.Summary.InputTokens)
	}
	if d2.Summary.TotalTokens != 930 { // 80+150+200+500
		t.Errorf("day 2 total: got %d, want 930", d2.Summary.TotalTokens)
	}
	if d2.Summary.EntryCount != 1 {
		t.Errorf("day 2 entry count: got %d, want 1", d2.Summary.EntryCount)
	}
}

func TestAggregateSessions(t *testing.T) {
	entries := loadSampleEntries(t)
	reports := AggregateSessions(entries)

	if len(reports) != 2 {
		t.Fatalf("expected 2 session reports, got %d", len(reports))
	}

	// Session 1 (2 entries, starts 10:00:05, ends 10:01:05)
	s1 := reports[0]
	if s1.SessionID != "test-session-1" {
		t.Errorf("session 1 id: got %q, want %q", s1.SessionID, "test-session-1")
	}
	wantStart, _ := time.Parse(time.RFC3339, "2026-03-24T10:00:05Z")
	wantEnd, _ := time.Parse(time.RFC3339, "2026-03-24T10:01:05Z")
	if !s1.StartTime.Equal(wantStart) {
		t.Errorf("session 1 start: got %v, want %v", s1.StartTime, wantStart)
	}
	if !s1.EndTime.Equal(wantEnd) {
		t.Errorf("session 1 end: got %v, want %v", s1.EndTime, wantEnd)
	}
	if s1.Summary.InputTokens != 150 {
		t.Errorf("session 1 input: got %d, want 150", s1.Summary.InputTokens)
	}
	if s1.Summary.TotalTokens != 4150 {
		t.Errorf("session 1 total: got %d, want 4150", s1.Summary.TotalTokens)
	}
	if s1.Summary.EntryCount != 2 {
		t.Errorf("session 1 entry count: got %d, want 2", s1.Summary.EntryCount)
	}
	if len(s1.Models) != 2 {
		t.Errorf("session 1 models: expected 2, got %d", len(s1.Models))
	}

	// Session 2 (1 entry)
	s2 := reports[1]
	if s2.SessionID != "test-session-2" {
		t.Errorf("session 2 id: got %q, want %q", s2.SessionID, "test-session-2")
	}
	if s2.Summary.EntryCount != 1 {
		t.Errorf("session 2 entry count: got %d, want 1", s2.Summary.EntryCount)
	}
	if s2.Summary.TotalTokens != 930 {
		t.Errorf("session 2 total: got %d, want 930", s2.Summary.TotalTokens)
	}
}

func TestAggregateBlocks(t *testing.T) {
	entries := loadSampleEntries(t)
	reports := AggregateBlocks(entries)

	// Entry 0 & 1: 2026-03-24T10:00:05 and T10:01:05 → block starting at 10:00 UTC (hour 10, block 10:00-15:00)
	// Entry 2: 2026-03-25T14:30:00 → block starting at 10:00 UTC (hour 14, block 10:00-15:00)
	// So we expect 2 blocks (different days).
	if len(reports) < 2 {
		t.Fatalf("expected at least 2 block reports, got %d", len(reports))
	}

	// First block should contain entries from 2026-03-24
	b1 := reports[0]
	if b1.Summary.EntryCount != 2 {
		t.Errorf("block 1 entry count: got %d, want 2", b1.Summary.EntryCount)
	}
	if b1.Summary.TotalTokens != 4150 {
		t.Errorf("block 1 total: got %d, want 4150", b1.Summary.TotalTokens)
	}

	// Block duration should be 5 hours
	blockDuration := b1.EndTime.Sub(b1.StartTime)
	if blockDuration != 5*time.Hour {
		t.Errorf("block duration: got %v, want 5h", blockDuration)
	}

	// Last block should contain entry from 2026-03-25
	bLast := reports[len(reports)-1]
	if bLast.Summary.EntryCount != 1 {
		t.Errorf("last block entry count: got %d, want 1", bLast.Summary.EntryCount)
	}
	if bLast.Summary.TotalTokens != 930 {
		t.Errorf("last block total: got %d, want 930", bLast.Summary.TotalTokens)
	}
}

func TestAggregateMonthly(t *testing.T) {
	entries := loadSampleEntries(t)
	reports := AggregateMonthly(entries)

	// All entries are in 2026-03, so expect 1 monthly report.
	if len(reports) != 1 {
		t.Fatalf("expected 1 monthly report, got %d", len(reports))
	}

	m1 := reports[0]
	if m1.Month != "2026-03" {
		t.Errorf("month: got %q, want %q", m1.Month, "2026-03")
	}
	// All 3 entries combined: input=230, output=650, cache_create=700, cache_read=3500
	if m1.Summary.InputTokens != 230 {
		t.Errorf("monthly input: got %d, want 230", m1.Summary.InputTokens)
	}
	if m1.Summary.OutputTokens != 650 {
		t.Errorf("monthly output: got %d, want 650", m1.Summary.OutputTokens)
	}
	if m1.Summary.CacheCreationInputTokens != 700 {
		t.Errorf("monthly cache_create: got %d, want 700", m1.Summary.CacheCreationInputTokens)
	}
	if m1.Summary.CacheReadInputTokens != 3500 {
		t.Errorf("monthly cache_read: got %d, want 3500", m1.Summary.CacheReadInputTokens)
	}
	if m1.Summary.TotalTokens != 5080 {
		t.Errorf("monthly total: got %d, want 5080", m1.Summary.TotalTokens)
	}
	if m1.Summary.EntryCount != 3 {
		t.Errorf("monthly entry count: got %d, want 3", m1.Summary.EntryCount)
	}
	// Verify per-model breakdown exists
	if len(m1.Models) != 2 {
		t.Errorf("monthly models: expected 2, got %d", len(m1.Models))
	}
}

func TestAggregateMonthlyMultiMonth(t *testing.T) {
	// Create entries spanning two months.
	entries := []Entry{
		{
			Model:     "claude-opus-4-6",
			Timestamp: time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC),
			Usage:     TokenUsage{InputTokens: 100, OutputTokens: 200},
		},
		{
			Model:     "claude-opus-4-6",
			Timestamp: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
			Usage:     TokenUsage{InputTokens: 300, OutputTokens: 400},
		},
	}

	reports := AggregateMonthly(entries)
	if len(reports) != 2 {
		t.Fatalf("expected 2 monthly reports, got %d", len(reports))
	}
	if reports[0].Month != "2026-02" {
		t.Errorf("first month: got %q, want %q", reports[0].Month, "2026-02")
	}
	if reports[1].Month != "2026-03" {
		t.Errorf("second month: got %q, want %q", reports[1].Month, "2026-03")
	}
	if reports[0].Summary.InputTokens != 100 {
		t.Errorf("feb input: got %d, want 100", reports[0].Summary.InputTokens)
	}
	if reports[1].Summary.InputTokens != 300 {
		t.Errorf("mar input: got %d, want 300", reports[1].Summary.InputTokens)
	}
}

func TestAggregateWeekly(t *testing.T) {
	entries := loadSampleEntries(t)
	reports := AggregateWeekly(entries)

	// 2026-03-24 is Monday of W13, 2026-03-25 is Tuesday of W13
	// All entries should be in the same ISO week.
	if len(reports) != 1 {
		t.Fatalf("expected 1 weekly report, got %d", len(reports))
	}

	w1 := reports[0]
	if w1.Week != "2026-W13" {
		t.Errorf("week: got %q, want %q", w1.Week, "2026-W13")
	}
	if w1.Summary.InputTokens != 230 {
		t.Errorf("weekly input: got %d, want 230", w1.Summary.InputTokens)
	}
	if w1.Summary.EntryCount != 3 {
		t.Errorf("weekly entry count: got %d, want 3", w1.Summary.EntryCount)
	}
	if len(w1.Models) != 2 {
		t.Errorf("weekly models: expected 2, got %d", len(w1.Models))
	}
}

func TestAggregateWeeklyMultiWeek(t *testing.T) {
	// Create entries spanning two ISO weeks.
	entries := []Entry{
		{
			Model:     "claude-opus-4-6",
			Timestamp: time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC), // Sunday = W12
			Usage:     TokenUsage{InputTokens: 100, OutputTokens: 200},
		},
		{
			Model:     "claude-opus-4-6",
			Timestamp: time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC), // Monday = W13
			Usage:     TokenUsage{InputTokens: 300, OutputTokens: 400},
		},
	}

	reports := AggregateWeekly(entries)
	if len(reports) != 2 {
		t.Fatalf("expected 2 weekly reports, got %d", len(reports))
	}
	if reports[0].Week != "2026-W12" {
		t.Errorf("first week: got %q, want %q", reports[0].Week, "2026-W12")
	}
	if reports[1].Week != "2026-W13" {
		t.Errorf("second week: got %q, want %q", reports[1].Week, "2026-W13")
	}
}

func TestFormatMonthlyTable(t *testing.T) {
	entries := loadSampleEntries(t)
	reports := AggregateMonthly(entries)
	table := FormatMonthlyTable(reports, false)

	for _, col := range []string{"Month", "Input", "Output", "Cache Read", "Cache Create", "Total", "Cost"} {
		if !strings.Contains(table, col) {
			t.Errorf("monthly table missing column header %q", col)
		}
	}
	if !strings.Contains(table, "2026-03") {
		t.Error("monthly table missing month 2026-03")
	}
	if !strings.Contains(table, "Total") {
		t.Error("monthly table missing Total row")
	}
}

func TestFormatWeeklyTable(t *testing.T) {
	entries := loadSampleEntries(t)
	reports := AggregateWeekly(entries)
	table := FormatWeeklyTable(reports, false)

	for _, col := range []string{"Week", "Input", "Output", "Cache Read", "Cache Create", "Total", "Cost"} {
		if !strings.Contains(table, col) {
			t.Errorf("weekly table missing column header %q", col)
		}
	}
	if !strings.Contains(table, "2026-W13") {
		t.Error("weekly table missing week 2026-W13")
	}
}

func TestFormatDailyTableWithBreakdown(t *testing.T) {
	entries := loadSampleEntries(t)
	reports := AggregateDailies(entries)
	table := FormatDailyTableWithBreakdown(reports, false)

	// Should contain model names as sub-rows
	if !strings.Contains(table, "claude-opus-4-6") {
		t.Error("breakdown table missing model claude-opus-4-6")
	}
	if !strings.Contains(table, "claude-sonnet-4-6") {
		t.Error("breakdown table missing model claude-sonnet-4-6")
	}
	// Should also contain date headers
	if !strings.Contains(table, "2026-03-24") {
		t.Error("breakdown table missing date 2026-03-24")
	}
}

func TestFormatDailyCSV(t *testing.T) {
	entries := loadSampleEntries(t)
	reports := AggregateDailies(entries)
	csv := FormatDailyCSV(reports)

	// Check header
	if !strings.HasPrefix(csv, "Date,Input,Output,CacheRead,CacheCreate,Total,Cost\n") {
		t.Errorf("CSV missing expected header, got: %s", csv[:60])
	}
	// Check data rows
	lines := strings.Split(strings.TrimSpace(csv), "\n")
	if len(lines) != 3 { // header + 2 data rows
		t.Errorf("CSV: expected 3 lines (header + 2 data), got %d", len(lines))
	}
	if !strings.HasPrefix(lines[1], "2026-03-24,") {
		t.Errorf("CSV first data row should start with 2026-03-24, got: %s", lines[1])
	}
}

func TestFormatDailyMarkdown(t *testing.T) {
	entries := loadSampleEntries(t)
	reports := AggregateDailies(entries)
	md := FormatDailyMarkdown(reports)

	// Check markdown table structure
	if !strings.Contains(md, "| Date |") {
		t.Error("markdown missing header row")
	}
	if !strings.Contains(md, "|---") {
		t.Error("markdown missing separator row")
	}
	if !strings.Contains(md, "| 2026-03-24 |") {
		t.Error("markdown missing data row for 2026-03-24")
	}
}

func TestFormatROI(t *testing.T) {
	roi := FormatROI(1000.0, 300.0, false)

	if !strings.Contains(roi, "ROI Summary") {
		t.Error("ROI missing header")
	}
	if !strings.Contains(roi, "$300.00") {
		t.Error("ROI missing subscription cost")
	}
	if !strings.Contains(roi, "$1000.00") {
		t.Error("ROI missing equivalent API cost")
	}
	if !strings.Contains(roi, "$700.00") {
		t.Error("ROI missing savings amount")
	}
	if !strings.Contains(roi, "70.0%") {
		t.Error("ROI missing savings percentage")
	}
}

func TestFormatROIZeroCost(t *testing.T) {
	roi := FormatROI(0, 300.0, false)
	if !strings.Contains(roi, "0.0%") {
		t.Error("ROI with zero cost should show 0.0%")
	}
}

func TestFormatDailyTable(t *testing.T) {
	entries := loadSampleEntries(t)
	reports := AggregateDailies(entries)
	table := FormatDailyTable(reports, false)

	// Check headers present
	for _, col := range []string{"Date", "Input", "Output", "Cache Read", "Cache Create", "Total", "Cost"} {
		if !strings.Contains(table, col) {
			t.Errorf("table missing column header %q", col)
		}
	}

	// Check dates present
	if !strings.Contains(table, "2026-03-24") {
		t.Error("table missing date 2026-03-24")
	}
	if !strings.Contains(table, "2026-03-25") {
		t.Error("table missing date 2026-03-25")
	}

	// Check total row present
	if !strings.Contains(table, "Total") {
		t.Error("table missing Total row")
	}

	// Check cost formatting (dollar sign present)
	if !strings.Contains(table, "$") {
		t.Error("table missing dollar sign in cost")
	}
}

func TestFormatDailyTableColor(t *testing.T) {
	entries := loadSampleEntries(t)
	reports := AggregateDailies(entries)
	table := FormatDailyTable(reports, true)

	// ANSI escape sequences must be present when color is enabled.
	if !strings.Contains(table, "\033[") {
		t.Error("colored table missing ANSI escape codes")
	}
}

func TestFormatSessionTable(t *testing.T) {
	entries := loadSampleEntries(t)
	reports := AggregateSessions(entries)
	table := FormatSessionTable(reports, false)

	// Check headers
	for _, col := range []string{"Session", "Start", "Duration", "Tokens", "Cost"} {
		if !strings.Contains(table, col) {
			t.Errorf("table missing column header %q", col)
		}
	}

	// Check session IDs are truncated or present
	if !strings.Contains(table, "test-session-1") && !strings.Contains(table, "test-ses") {
		t.Error("table missing session 1 reference")
	}
}

func TestFormatBlockTable(t *testing.T) {
	entries := loadSampleEntries(t)
	reports := AggregateBlocks(entries)
	table := FormatBlockTable(reports, false)

	// Check headers
	for _, col := range []string{"Block Start", "Block End", "Tokens", "Cost"} {
		if !strings.Contains(table, col) {
			t.Errorf("table missing column header %q", col)
		}
	}
}

func TestFormatJSON(t *testing.T) {
	entries := loadSampleEntries(t)
	reports := AggregateDailies(entries)

	jsonStr, err := FormatJSON(reports)
	if err != nil {
		t.Fatalf("FormatJSON error: %v", err)
	}

	// Should be valid JSON
	var parsed []map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if len(parsed) != 2 {
		t.Errorf("expected 2 entries in JSON, got %d", len(parsed))
	}

	// Check expected fields
	first := parsed[0]
	if _, ok := first["Date"]; !ok {
		t.Error("JSON missing 'Date' field")
	}
	if _, ok := first["Summary"]; !ok {
		t.Error("JSON missing 'Summary' field")
	}
	if _, ok := first["Models"]; !ok {
		t.Error("JSON missing 'Models' field")
	}
}
