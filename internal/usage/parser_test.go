package usage

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseFile_Sample(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "usage_sample.jsonl")
	var entries []Entry
	err := ParseFile(path, func(e Entry) {
		entries = append(entries, e)
	})
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Entry 0: opus, session-1
	e := entries[0]
	if e.Model != "claude-opus-4-6" {
		t.Errorf("entry 0 model: got %q, want %q", e.Model, "claude-opus-4-6")
	}
	if e.SessionID != "test-session-1" {
		t.Errorf("entry 0 sessionID: got %q, want %q", e.SessionID, "test-session-1")
	}
	if e.Usage.InputTokens != 100 {
		t.Errorf("entry 0 input_tokens: got %d, want 100", e.Usage.InputTokens)
	}
	if e.Usage.OutputTokens != 200 {
		t.Errorf("entry 0 output_tokens: got %d, want 200", e.Usage.OutputTokens)
	}
	if e.Usage.CacheCreationInputTokens != 500 {
		t.Errorf("entry 0 cache_creation: got %d, want 500", e.Usage.CacheCreationInputTokens)
	}
	if e.Usage.CacheReadInputTokens != 1000 {
		t.Errorf("entry 0 cache_read: got %d, want 1000", e.Usage.CacheReadInputTokens)
	}
	wantTime, _ := time.Parse(time.RFC3339Nano, "2026-03-24T10:00:05.000Z")
	if !e.Timestamp.Equal(wantTime) {
		t.Errorf("entry 0 timestamp: got %v, want %v", e.Timestamp, wantTime)
	}

	// Entry 1: sonnet, session-1
	e = entries[1]
	if e.Model != "claude-sonnet-4-6" {
		t.Errorf("entry 1 model: got %q, want %q", e.Model, "claude-sonnet-4-6")
	}
	if e.Usage.InputTokens != 50 {
		t.Errorf("entry 1 input_tokens: got %d, want 50", e.Usage.InputTokens)
	}
	if e.Usage.OutputTokens != 300 {
		t.Errorf("entry 1 output_tokens: got %d, want 300", e.Usage.OutputTokens)
	}
	if e.Usage.CacheReadInputTokens != 2000 {
		t.Errorf("entry 1 cache_read: got %d, want 2000", e.Usage.CacheReadInputTokens)
	}

	// Entry 2: opus, session-2
	e = entries[2]
	if e.Model != "claude-opus-4-6" {
		t.Errorf("entry 2 model: got %q, want %q", e.Model, "claude-opus-4-6")
	}
	if e.SessionID != "test-session-2" {
		t.Errorf("entry 2 sessionID: got %q, want %q", e.SessionID, "test-session-2")
	}
	if e.Usage.InputTokens != 80 {
		t.Errorf("entry 2 input_tokens: got %d, want 80", e.Usage.InputTokens)
	}
}

func TestParseFile_SkipsNonAssistant(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "usage_sample.jsonl")
	count := 0
	err := ParseFile(path, func(e Entry) {
		count++
		if e.Model == "" {
			t.Error("received entry with empty model (non-assistant leaked through)")
		}
	})
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 assistant entries, got %d", count)
	}
}

func TestParseFile_HandlesCorruptLine(t *testing.T) {
	// Create a temp file with a corrupt line
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.jsonl")
	content := `{"type":"assistant","message":{"model":"claude-opus-4-6","usage":{"input_tokens":10,"output_tokens":20,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"timestamp":"2026-03-24T10:00:00.000Z","sessionId":"s1"}
{this is not valid json and type assistant
{"type":"assistant","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":5,"output_tokens":10,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"timestamp":"2026-03-24T10:01:00.000Z","sessionId":"s1"}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	var entries []Entry
	err := ParseFile(path, func(e Entry) {
		entries = append(entries, e)
	})
	if err != nil {
		t.Fatalf("ParseFile should not return error on corrupt lines, got: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 valid entries (skipping corrupt), got %d", len(entries))
	}
}

func TestParseFile_SkipsNoUsage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no_usage.jsonl")
	content := `{"type":"assistant","message":{"model":"claude-opus-4-6"},"timestamp":"2026-03-24T10:00:00.000Z","sessionId":"s1"}
{"type":"assistant","message":{"model":"claude-opus-4-6","usage":{"input_tokens":10,"output_tokens":20,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"timestamp":"2026-03-24T10:01:00.000Z","sessionId":"s1"}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	count := 0
	err := ParseFile(path, func(e Entry) {
		count++
	})
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 entry (skip no-usage), got %d", count)
	}
}

func TestCalculateCost_Opus(t *testing.T) {
	u := TokenUsage{
		InputTokens:              1_000_000,
		OutputTokens:             1_000_000,
		CacheCreationInputTokens: 1_000_000,
		CacheReadInputTokens:     1_000_000,
	}
	cost := CalculateCost("claude-opus-4-6", u)
	// 15.0 + 75.0 + 18.75 + 1.5 = 110.25
	want := 110.25
	if math.Abs(cost-want) > 0.001 {
		t.Errorf("CalculateCost opus: got %.4f, want %.4f", cost, want)
	}
}

func TestCalculateCost_Sonnet(t *testing.T) {
	u := TokenUsage{
		InputTokens:              1_000_000,
		OutputTokens:             1_000_000,
		CacheCreationInputTokens: 1_000_000,
		CacheReadInputTokens:     1_000_000,
	}
	cost := CalculateCost("claude-sonnet-4-6", u)
	// 3.0 + 15.0 + 3.75 + 0.3 = 22.05
	want := 22.05
	if math.Abs(cost-want) > 0.001 {
		t.Errorf("CalculateCost sonnet: got %.4f, want %.4f", cost, want)
	}
}

func TestCalculateCost_UnknownModel(t *testing.T) {
	u := TokenUsage{
		InputTokens:  1_000_000,
		OutputTokens: 1_000_000,
	}
	cost := CalculateCost("some-unknown-model", u)
	// Falls back to sonnet pricing: 3.0 + 15.0 = 18.0
	want := 18.0
	if math.Abs(cost-want) > 0.001 {
		t.Errorf("CalculateCost unknown: got %.4f, want %.4f (should use sonnet fallback)", cost, want)
	}
}

func TestScanDir(t *testing.T) {
	// Create a temp directory mimicking Claude projects structure
	dir := t.TempDir()
	projDir := filepath.Join(dir, "projects", "test-project")
	subagentDir := filepath.Join(projDir, "session-1", "subagents")
	if err := os.MkdirAll(subagentDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Main session file
	mainContent := `{"type":"assistant","message":{"model":"claude-opus-4-6","usage":{"input_tokens":100,"output_tokens":200,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"timestamp":"2026-03-24T10:00:00.000Z","sessionId":"s1"}
`
	if err := os.WriteFile(filepath.Join(projDir, "session-1.jsonl"), []byte(mainContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Subagent file (should also be included)
	subContent := `{"type":"assistant","message":{"model":"claude-haiku-4-5-20251001","usage":{"input_tokens":50,"output_tokens":100,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"timestamp":"2026-03-24T10:01:00.000Z","sessionId":"s1"}
`
	if err := os.WriteFile(filepath.Join(subagentDir, "agent-abc.jsonl"), []byte(subContent), 0644); err != nil {
		t.Fatal(err)
	}

	var entries []Entry
	err := ScanDir(dir, func(e Entry) {
		entries = append(entries, e)
	})
	if err != nil {
		t.Fatalf("ScanDir returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries (main + subagent), got %d", len(entries))
	}
}
