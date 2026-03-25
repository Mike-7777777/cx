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
