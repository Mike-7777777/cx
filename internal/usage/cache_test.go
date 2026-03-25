package usage

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUsageCache_LoadAndSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	// Create and populate a cache.
	cache, err := LoadUsageCache(path)
	if err != nil {
		t.Fatalf("LoadUsageCache (new): %v", err)
	}
	if cache.Version != cacheVersion {
		t.Errorf("version: got %d, want %d", cache.Version, cacheVersion)
	}

	cache.Files["test/file.jsonl"] = FileState{
		Size:      12345,
		MtimeUnix: 1711296000,
		Offset:    10000,
	}
	cache.Daily["2026-03-24"] = CachedSummary{
		InputTokens:  100,
		OutputTokens: 200,
		TotalTokens:  300,
		CostUSD:      0.05,
		EntryCount:   2,
		Models: map[string]CachedSummary{
			"claude-opus-4-6": {
				InputTokens:  100,
				OutputTokens: 200,
				TotalTokens:  300,
				CostUSD:      0.05,
				EntryCount:   2,
			},
		},
	}

	if err := cache.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Reload and verify.
	loaded, err := LoadUsageCache(path)
	if err != nil {
		t.Fatalf("LoadUsageCache (reload): %v", err)
	}
	if loaded.Version != cacheVersion {
		t.Errorf("loaded version: got %d, want %d", loaded.Version, cacheVersion)
	}

	fs, ok := loaded.Files["test/file.jsonl"]
	if !ok {
		t.Fatal("loaded cache missing file entry")
	}
	if fs.Size != 12345 || fs.MtimeUnix != 1711296000 || fs.Offset != 10000 {
		t.Errorf("file state mismatch: %+v", fs)
	}

	ds, ok := loaded.Daily["2026-03-24"]
	if !ok {
		t.Fatal("loaded cache missing daily entry")
	}
	if ds.InputTokens != 100 || ds.OutputTokens != 200 || ds.EntryCount != 2 {
		t.Errorf("daily summary mismatch: %+v", ds)
	}
	if _, ok := ds.Models["claude-opus-4-6"]; !ok {
		t.Error("daily summary missing model entry")
	}
}

func TestUsageCache_LoadCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	cache, err := LoadUsageCache(path)
	if err != nil {
		t.Fatalf("LoadUsageCache should not error on corrupt: %v", err)
	}
	// Should return empty cache.
	if len(cache.Files) != 0 || len(cache.Daily) != 0 {
		t.Error("corrupt cache should result in empty state")
	}
}

func TestUsageCache_LoadVersionMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	if err := os.WriteFile(path, []byte(`{"version":999,"files":{},"daily":{}}`), 0644); err != nil {
		t.Fatal(err)
	}

	cache, err := LoadUsageCache(path)
	if err != nil {
		t.Fatalf("LoadUsageCache should not error on version mismatch: %v", err)
	}
	if cache.Version != cacheVersion {
		t.Errorf("should reset to current version, got %d", cache.Version)
	}
}

// mockFileInfo implements os.FileInfo for testing NeedsUpdate.
type mockFileInfo struct {
	size    int64
	modTime time.Time
}

func (m mockFileInfo) Name() string      { return "test.jsonl" }
func (m mockFileInfo) Size() int64       { return m.size }
func (m mockFileInfo) Mode() os.FileMode { return 0644 }
func (m mockFileInfo) ModTime() time.Time { return m.modTime }
func (m mockFileInfo) IsDir() bool       { return false }
func (m mockFileInfo) Sys() any          { return nil }

func TestUsageCache_NeedsUpdate_NewFile(t *testing.T) {
	cache := &UsageCache{Files: make(map[string]FileState)}
	info := mockFileInfo{size: 1000, modTime: time.Unix(1711296000, 0)}

	if !cache.NeedsUpdate("new/file.jsonl", info) {
		t.Error("new file should need update")
	}
}

func TestUsageCache_NeedsUpdate_Unchanged(t *testing.T) {
	cache := &UsageCache{Files: map[string]FileState{
		"existing.jsonl": {Size: 1000, MtimeUnix: 1711296000, Offset: 1000},
	}}
	info := mockFileInfo{size: 1000, modTime: time.Unix(1711296000, 0)}

	if cache.NeedsUpdate("existing.jsonl", info) {
		t.Error("unchanged file should not need update")
	}
}

func TestUsageCache_NeedsUpdate_SizeChanged(t *testing.T) {
	cache := &UsageCache{Files: map[string]FileState{
		"existing.jsonl": {Size: 1000, MtimeUnix: 1711296000, Offset: 1000},
	}}
	info := mockFileInfo{size: 2000, modTime: time.Unix(1711296000, 0)}

	if !cache.NeedsUpdate("existing.jsonl", info) {
		t.Error("file with changed size should need update")
	}
}

func TestUsageCache_NeedsUpdate_MtimeChanged(t *testing.T) {
	cache := &UsageCache{Files: map[string]FileState{
		"existing.jsonl": {Size: 1000, MtimeUnix: 1711296000, Offset: 1000},
	}}
	info := mockFileInfo{size: 1000, modTime: time.Unix(1711296999, 0)}

	if !cache.NeedsUpdate("existing.jsonl", info) {
		t.Error("file with changed mtime should need update")
	}
}

func TestUsageCache_NeedsUpdate_Truncated(t *testing.T) {
	cache := &UsageCache{Files: map[string]FileState{
		"existing.jsonl": {Size: 500, MtimeUnix: 1711296000, Offset: 1000},
	}}
	// File is now 500 bytes but offset was 1000 → truncated.
	info := mockFileInfo{size: 500, modTime: time.Unix(1711296000, 0)}

	if !cache.NeedsUpdate("existing.jsonl", info) {
		t.Error("truncated file should need update")
	}
}

func TestUsageCache_MergeDailyEntry(t *testing.T) {
	cache := &UsageCache{
		Files: make(map[string]FileState),
		Daily: make(map[string]CachedSummary),
	}

	e := Entry{
		Model: "claude-opus-4-6",
		Usage: TokenUsage{
			InputTokens:              100,
			OutputTokens:             200,
			CacheCreationInputTokens: 500,
			CacheReadInputTokens:     1000,
		},
		Timestamp: time.Date(2026, 3, 24, 10, 0, 0, 0, time.UTC),
	}

	cache.MergeDailyEntry("2026-03-24", e)

	ds := cache.Daily["2026-03-24"]
	if ds.InputTokens != 100 {
		t.Errorf("input: got %d, want 100", ds.InputTokens)
	}
	if ds.OutputTokens != 200 {
		t.Errorf("output: got %d, want 200", ds.OutputTokens)
	}
	if ds.TotalTokens != 1800 { // 100+200+500+1000
		t.Errorf("total: got %d, want 1800", ds.TotalTokens)
	}
	if ds.EntryCount != 1 {
		t.Errorf("count: got %d, want 1", ds.EntryCount)
	}
	if len(ds.Models) != 1 {
		t.Fatalf("models: got %d, want 1", len(ds.Models))
	}
	ms := ds.Models["claude-opus-4-6"]
	if ms.InputTokens != 100 || ms.OutputTokens != 200 {
		t.Errorf("model tokens mismatch: %+v", ms)
	}

	// Merge a second entry on the same day.
	e2 := Entry{
		Model: "claude-sonnet-4-6",
		Usage: TokenUsage{
			InputTokens:  50,
			OutputTokens: 300,
		},
		Timestamp: time.Date(2026, 3, 24, 11, 0, 0, 0, time.UTC),
	}
	cache.MergeDailyEntry("2026-03-24", e2)

	ds = cache.Daily["2026-03-24"]
	if ds.EntryCount != 2 {
		t.Errorf("count after merge: got %d, want 2", ds.EntryCount)
	}
	if ds.InputTokens != 150 {
		t.Errorf("input after merge: got %d, want 150", ds.InputTokens)
	}
	if len(ds.Models) != 2 {
		t.Errorf("models after merge: got %d, want 2", len(ds.Models))
	}
}

func TestUsageCache_DailyReports(t *testing.T) {
	cache := &UsageCache{
		Files: make(map[string]FileState),
		Daily: map[string]CachedSummary{
			"2026-03-24": {
				InputTokens:  100,
				OutputTokens: 200,
				TotalTokens:  300,
				CostUSD:      0.05,
				EntryCount:   2,
				Models: map[string]CachedSummary{
					"claude-opus-4-6": {InputTokens: 100, OutputTokens: 200, TotalTokens: 300, CostUSD: 0.05, EntryCount: 2},
				},
			},
		},
	}

	reports := cache.DailyReports()
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}

	r := reports[0]
	if r.Date != "2026-03-24" {
		t.Errorf("date: got %q, want %q", r.Date, "2026-03-24")
	}
	if r.Summary.InputTokens != 100 {
		t.Errorf("input: got %d, want 100", r.Summary.InputTokens)
	}
	if r.Summary.EntryCount != 2 {
		t.Errorf("count: got %d, want 2", r.Summary.EntryCount)
	}
	if _, ok := r.Models["claude-opus-4-6"]; !ok {
		t.Error("missing model in report")
	}
}

func TestParseFileFrom_Offset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	line1 := `{"type":"assistant","message":{"model":"claude-opus-4-6","usage":{"input_tokens":100,"output_tokens":200,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"timestamp":"2026-03-24T10:00:00.000Z","sessionId":"s1"}` + "\n"
	line2 := `{"type":"assistant","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":50,"output_tokens":300,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"timestamp":"2026-03-24T10:01:00.000Z","sessionId":"s1"}` + "\n"

	content := line1 + line2
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Parse from start: should get 2 entries.
	var allEntries []Entry
	endOffset, err := ParseFileFrom(path, 0, func(e Entry) {
		allEntries = append(allEntries, e)
	})
	if err != nil {
		t.Fatalf("ParseFileFrom(0): %v", err)
	}
	if len(allEntries) != 2 {
		t.Fatalf("from offset 0: got %d entries, want 2", len(allEntries))
	}
	if endOffset != int64(len(content)) {
		t.Errorf("end offset: got %d, want %d", endOffset, len(content))
	}

	// Parse from offset after line1: should get only entry 2.
	offset1 := int64(len(line1))
	var secondOnly []Entry
	endOffset2, err := ParseFileFrom(path, offset1, func(e Entry) {
		secondOnly = append(secondOnly, e)
	})
	if err != nil {
		t.Fatalf("ParseFileFrom(%d): %v", offset1, err)
	}
	if len(secondOnly) != 1 {
		t.Fatalf("from offset %d: got %d entries, want 1", offset1, len(secondOnly))
	}
	if secondOnly[0].Model != "claude-sonnet-4-6" {
		t.Errorf("entry model: got %q, want %q", secondOnly[0].Model, "claude-sonnet-4-6")
	}
	if endOffset2 != int64(len(content)) {
		t.Errorf("end offset 2: got %d, want %d", endOffset2, len(content))
	}

	// Parse from end of file: should get no entries.
	var none []Entry
	endOffset3, err := ParseFileFrom(path, int64(len(content)), func(e Entry) {
		none = append(none, e)
	})
	if err != nil {
		t.Fatalf("ParseFileFrom(end): %v", err)
	}
	if len(none) != 0 {
		t.Errorf("from end: got %d entries, want 0", len(none))
	}
	if endOffset3 != int64(len(content)) {
		t.Errorf("end offset 3: got %d, want %d", endOffset3, len(content))
	}
}

func TestScanDirCached(t *testing.T) {
	// Create a temp directory mimicking Claude projects structure.
	dir := t.TempDir()
	projDir := filepath.Join(dir, "projects", "test-project")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}

	line1 := `{"type":"assistant","message":{"model":"claude-opus-4-6","usage":{"input_tokens":100,"output_tokens":200,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"timestamp":"2026-03-24T10:00:00.000Z","sessionId":"s1"}` + "\n"

	sessionFile := filepath.Join(projDir, "session-1.jsonl")
	if err := os.WriteFile(sessionFile, []byte(line1), 0644); err != nil {
		t.Fatal(err)
	}

	// First scan: should pick up the entry.
	cache := &UsageCache{
		Version: cacheVersion,
		Files:   make(map[string]FileState),
		Daily:   make(map[string]CachedSummary),
	}
	var entries []Entry
	if err := ScanDirCached(dir, cache, func(e Entry) {
		entries = append(entries, e)
	}); err != nil {
		t.Fatalf("ScanDirCached (first): %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("first scan: got %d entries, want 1", len(entries))
	}

	// Second scan (no changes): should get 0 new entries.
	var entriesSecond []Entry
	if err := ScanDirCached(dir, cache, func(e Entry) {
		entriesSecond = append(entriesSecond, e)
	}); err != nil {
		t.Fatalf("ScanDirCached (second): %v", err)
	}
	if len(entriesSecond) != 0 {
		t.Errorf("second scan: got %d entries, want 0 (unchanged files)", len(entriesSecond))
	}

	// Append a new line and scan again: should get only the new entry.
	line2 := `{"type":"assistant","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":50,"output_tokens":300,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"timestamp":"2026-03-24T10:01:00.000Z","sessionId":"s1"}` + "\n"
	f, err := os.OpenFile(sessionFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(line2); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	var entriesThird []Entry
	if err := ScanDirCached(dir, cache, func(e Entry) {
		entriesThird = append(entriesThird, e)
	}); err != nil {
		t.Fatalf("ScanDirCached (third): %v", err)
	}
	if len(entriesThird) != 1 {
		t.Fatalf("third scan: got %d entries, want 1 (only new data)", len(entriesThird))
	}
	if entriesThird[0].Model != "claude-sonnet-4-6" {
		t.Errorf("new entry model: got %q, want %q", entriesThird[0].Model, "claude-sonnet-4-6")
	}
}

func TestScanDirCached_CostAccuracy(t *testing.T) {
	// Verify that cached daily summaries produce the same cost as fresh aggregation.
	dir := t.TempDir()
	projDir := filepath.Join(dir, "projects", "test-project")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `{"type":"assistant","message":{"model":"claude-opus-4-6","usage":{"input_tokens":100,"output_tokens":200,"cache_creation_input_tokens":500,"cache_read_input_tokens":1000}},"timestamp":"2026-03-24T10:00:00.000Z","sessionId":"s1"}
{"type":"assistant","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":50,"output_tokens":300,"cache_creation_input_tokens":0,"cache_read_input_tokens":2000}},"timestamp":"2026-03-24T10:01:00.000Z","sessionId":"s1"}
`
	if err := os.WriteFile(filepath.Join(projDir, "session-1.jsonl"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Scan with cache.
	cache := &UsageCache{
		Version: cacheVersion,
		Files:   make(map[string]FileState),
		Daily:   make(map[string]CachedSummary),
	}
	ScanDirCached(dir, cache, func(e Entry) {
		dateKey := e.Timestamp.UTC().Format("2006-01-02")
		cache.MergeDailyEntry(dateKey, e)
	})

	// Also do a fresh scan for comparison.
	var freshEntries []Entry
	ScanDir(dir, func(e Entry) {
		freshEntries = append(freshEntries, e)
	})
	freshReports := AggregateDailies(freshEntries)

	cachedReports := cache.DailyReports()
	if len(cachedReports) != len(freshReports) {
		t.Fatalf("report count: cached=%d, fresh=%d", len(cachedReports), len(freshReports))
	}

	for _, fr := range freshReports {
		ds, ok := cache.Daily[fr.Date]
		if !ok {
			t.Errorf("cached missing date %s", fr.Date)
			continue
		}
		if ds.InputTokens != fr.Summary.InputTokens {
			t.Errorf("date %s input: cached=%d, fresh=%d", fr.Date, ds.InputTokens, fr.Summary.InputTokens)
		}
		if ds.OutputTokens != fr.Summary.OutputTokens {
			t.Errorf("date %s output: cached=%d, fresh=%d", fr.Date, ds.OutputTokens, fr.Summary.OutputTokens)
		}
		if ds.TotalTokens != fr.Summary.TotalTokens {
			t.Errorf("date %s total: cached=%d, fresh=%d", fr.Date, ds.TotalTokens, fr.Summary.TotalTokens)
		}
		if math.Abs(ds.CostUSD-fr.Summary.CostUSD) > 0.0001 {
			t.Errorf("date %s cost: cached=%.6f, fresh=%.6f", fr.Date, ds.CostUSD, fr.Summary.CostUSD)
		}
		if ds.EntryCount != fr.Summary.EntryCount {
			t.Errorf("date %s count: cached=%d, fresh=%d", fr.Date, ds.EntryCount, fr.Summary.EntryCount)
		}
	}
}
