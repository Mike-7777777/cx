package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mike-7777777/cx/internal/config"
)

func TestInsightsCmd_OutputContainsSections(t *testing.T) {
	// Create a temp dir with the projects/ layout that ScanDir expects.
	tmp := t.TempDir()
	projDir := filepath.Join(tmp, "projects", "test-project")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("creating project dir: %v", err)
	}

	// Write a JSONL file with 2 assistant entries.
	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":100,"output_tokens":200,"cache_creation_input_tokens":50,"cache_read_input_tokens":300}},"timestamp":"2026-03-24T10:00:05.000Z","sessionId":"test-session-1"}
{"type":"assistant","message":{"model":"claude-opus-4-6","usage":{"input_tokens":80,"output_tokens":150,"cache_creation_input_tokens":0,"cache_read_input_tokens":500}},"timestamp":"2026-03-24T14:30:00.000Z","sessionId":"test-session-2"}
`
	if err := os.WriteFile(filepath.Join(projDir, "test.jsonl"), []byte(jsonl), 0o644); err != nil {
		t.Fatalf("writing jsonl: %v", err)
	}

	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{Main: "main", Accounts: map[string]config.Account{
			"main": {ConfigDir: tmp},
		}},
		Stdout:   &buf,
		Stderr:   &buf,
		UseColor: false,
	}

	cmd := &insightsCmd{}
	if err := cmd.Run(context.Background(), app, []string{"--dir", tmp}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	got := buf.String()
	for _, want := range []string{"Hourly Distribution", "Model Distribution", "Efficiency"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\ngot:\n%s", want, got)
		}
	}
}

func TestInsightsCmd_JSONOutput(t *testing.T) {
	tmp := t.TempDir()
	projDir := filepath.Join(tmp, "projects", "test-project")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("creating project dir: %v", err)
	}

	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":100,"output_tokens":200,"cache_creation_input_tokens":50,"cache_read_input_tokens":300}},"timestamp":"2026-03-24T10:00:05.000Z","sessionId":"test-session-1"}
{"type":"assistant","message":{"model":"claude-opus-4-6","usage":{"input_tokens":80,"output_tokens":150,"cache_creation_input_tokens":0,"cache_read_input_tokens":500}},"timestamp":"2026-03-24T14:30:00.000Z","sessionId":"test-session-2"}
`
	if err := os.WriteFile(filepath.Join(projDir, "test.jsonl"), []byte(jsonl), 0o644); err != nil {
		t.Fatalf("writing jsonl: %v", err)
	}

	var stdout, stderr bytes.Buffer
	app := &App{
		Registry: &config.Registry{Main: "main", Accounts: map[string]config.Account{
			"main": {ConfigDir: tmp},
		}},
		Stdout:   &stdout,
		Stderr:   &stderr,
		UseColor: false,
	}

	cmd := &insightsCmd{}
	if err := cmd.Run(context.Background(), app, []string{"--json", "--dir", tmp}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var report map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot:\n%s", err, stdout.String())
	}

	for _, key := range []string{"hourly", "models", "projects", "efficiency"} {
		if _, ok := report[key]; !ok {
			t.Errorf("JSON output missing key %q\ngot keys: %v", key, keysOf(report))
		}
	}
}

func TestInsightsCmd_SinceFilter(t *testing.T) {
	tmp := t.TempDir()
	projDir := filepath.Join(tmp, "projects", "test-project")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("creating project dir: %v", err)
	}

	// Two entries: one old (2026-03-20), one recent (2026-03-25).
	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":100,"output_tokens":200,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"timestamp":"2026-03-20T10:00:00.000Z","sessionId":"old-session"}
{"type":"assistant","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":50,"output_tokens":80,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"timestamp":"2026-03-25T10:00:00.000Z","sessionId":"recent-session"}
`
	if err := os.WriteFile(filepath.Join(projDir, "test.jsonl"), []byte(jsonl), 0o644); err != nil {
		t.Fatalf("writing jsonl: %v", err)
	}

	var stdout, stderr bytes.Buffer
	app := &App{
		Registry: &config.Registry{Main: "main", Accounts: map[string]config.Account{
			"main": {ConfigDir: tmp},
		}},
		Stdout:   &stdout,
		Stderr:   &stderr,
		UseColor: false,
	}

	cmd := &insightsCmd{}
	// Filter to only entries on or after 2026-03-24; only the recent entry should survive.
	if err := cmd.Run(context.Background(), app, []string{"--json", "--dir", tmp, "--since", "2026-03-24"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var report insightsReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot:\n%s", err, stdout.String())
	}

	// After the filter only the recent entry survives, so exactly one model appears.
	if len(report.Models) != 1 {
		t.Errorf("expected 1 model after --since filter, got %d: %+v", len(report.Models), report.Models)
	}
	// The surviving entry has 50 input + 80 output = 130 tokens; AvgTokensPerMsg must be 130.
	if report.Efficiency.AvgTokensPerMsg != 130 {
		t.Errorf("expected AvgTokensPerMsg=130 after --since filter, got %d", report.Efficiency.AvgTokensPerMsg)
	}
}

func TestInsightsCmd_EmptyDir(t *testing.T) {
	tmp := t.TempDir()
	// Create the projects directory but leave it empty (no project subdirs).
	if err := os.MkdirAll(filepath.Join(tmp, "projects"), 0o755); err != nil {
		t.Fatalf("creating projects dir: %v", err)
	}

	var stdout, stderr bytes.Buffer
	app := &App{
		Registry: &config.Registry{Main: "main", Accounts: map[string]config.Account{
			"main": {ConfigDir: tmp},
		}},
		Stdout:   &stdout,
		Stderr:   &stderr,
		UseColor: false,
	}

	cmd := &insightsCmd{}
	if err := cmd.Run(context.Background(), app, []string{"--dir", tmp}); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if !strings.Contains(stderr.String(), "no usage entries") {
		t.Errorf("expected stderr to contain %q\ngot stderr: %q\ngot stdout: %q",
			"no usage entries", stderr.String(), stdout.String())
	}
}

func TestInsightsCmd_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	app := &App{
		Registry: &config.Registry{Main: "main", Accounts: map[string]config.Account{}},
		Stdout:   &stdout,
		Stderr:   &stderr,
		UseColor: false,
	}

	cmd := &insightsCmd{}
	if err := cmd.Run(context.Background(), app, []string{"--help"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "cx insights") {
		t.Errorf("help output missing %q\ngot:\n%s", "cx insights", stdout.String())
	}
}

// keysOf returns the keys of a map as a slice, for error messages.
func keysOf(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
