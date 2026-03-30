package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Mike-7777777/cx/internal/cache"
	"github.com/Mike-7777777/cx/internal/usage"
)

func TestFiveHourStats_NoFile(t *testing.T) {
	dir := t.TempDir()
	pct, ttr, rc, ok := fiveHourStats(dir)
	if ok {
		t.Error("expected ok=false for missing rate-cache")
	}
	if pct != 0 || ttr != 0 {
		t.Errorf("expected zero values, got pct=%f ttr=%v", pct, ttr)
	}
	_ = rc // rc may be nil, that's fine
}

func TestFiveHourStats_WithData(t *testing.T) {
	dir := t.TempDir()
	resetsAt := time.Now().Add(2 * time.Hour).Unix()
	rc := &cache.RateCache{
		Account:   "test",
		UpdatedAt: time.Now().Unix(),
		RateLimits: &cache.RateLimits{
			FiveHour: &cache.Window{
				UsedPercentage: 42.0,
				ResetsAt:       resetsAt,
			},
		},
	}
	if err := cache.WriteRateCache(filepath.Join(dir, "rate-cache.json"), rc); err != nil {
		t.Fatalf("WriteRateCache: %v", err)
	}

	pct, ttr, _, ok := fiveHourStats(dir)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if pct != 42.0 {
		t.Errorf("pct=%f, want 42.0", pct)
	}
	if ttr <= 0 {
		t.Errorf("ttr=%v, want positive duration", ttr)
	}
}

func TestSevenDayHeadroomFromCache_Nil(t *testing.T) {
	h := sevenDayHeadroomFromCache(nil)
	if h != 1.0 {
		t.Errorf("expected 1.0 for nil cache, got %f", h)
	}
}

func TestSevenDayHeadroomFromCache_NoSevenDay(t *testing.T) {
	rc := &cache.RateCache{
		RateLimits: &cache.RateLimits{},
	}
	h := sevenDayHeadroomFromCache(rc)
	if h != 1.0 {
		t.Errorf("expected 1.0 for missing 7d data, got %f", h)
	}
}

func TestParseSinceDate_Valid(t *testing.T) {
	tm, err := parseSinceDate("2026-03-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tm.Year() != 2026 || tm.Month() != 3 || tm.Day() != 15 {
		t.Errorf("parsed date wrong: %v", tm)
	}
}

func TestParseSinceDate_Empty(t *testing.T) {
	tm, err := parseSinceDate("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tm.IsZero() {
		t.Errorf("expected zero time for empty string, got %v", tm)
	}
}

func TestParseSinceDate_Invalid(t *testing.T) {
	_, err := parseSinceDate("not-a-date")
	if err == nil {
		t.Error("expected error for invalid date")
	}
}

func TestFilterEntriesSince_ZeroTime(t *testing.T) {
	entries := []usage.Entry{{Timestamp: time.Now()}}
	result := filterEntriesSince(entries, time.Time{})
	if len(result) != 1 {
		t.Errorf("zero time should return all entries, got %d", len(result))
	}
}

func TestFilterEntriesSince_Filters(t *testing.T) {
	cutoff := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	entries := []usage.Entry{
		{Timestamp: time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)},
		{Timestamp: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)},
		{Timestamp: time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)},
	}
	result := filterEntriesSince(entries, cutoff)
	if len(result) != 2 {
		t.Errorf("expected 2 entries after cutoff, got %d", len(result))
	}
}

func TestFilterEntriesSince_Empty(t *testing.T) {
	result := filterEntriesSince(nil, time.Now())
	if len(result) != 0 {
		t.Errorf("expected 0, got %d", len(result))
	}
}
