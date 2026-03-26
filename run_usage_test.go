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

// sampleJSONL is a minimal valid JSONL entry for usage tests.
const sampleJSONL = `{"type":"assistant","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":100,"output_tokens":200,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"timestamp":"2026-03-24T10:00:05.000Z","sessionId":"test-session-1"}
`

// setupUsageTempDir creates a temp claude config dir with one project JSONL file.
// Returns the config dir path.
func setupUsageTempDir(t *testing.T) string {
	t.Helper()
	configDir := t.TempDir()
	projDir := filepath.Join(configDir, "projects", "test-project")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	jsonlPath := filepath.Join(projDir, "session-1.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(sampleJSONL), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	return configDir
}

func TestUsage_NoArgs(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{Main: "main", Accounts: map[string]config.Account{
			"main": {ConfigDir: t.TempDir()},
		}},
		Stdout: &buf, Stderr: &buf, UseColor: false,
	}
	cmd := &usageCmd{}
	// With no args, defaults to "daily" mode and attempts config detection.
	// In a test environment this will either error or produce no entries.
	// Either outcome is acceptable — the key assertion is no panic.
	_ = cmd.Run(context.Background(), app, nil)
}

func TestUsage_UnknownMode(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{Main: "main", Accounts: map[string]config.Account{}},
		Stdout:   &buf, Stderr: &buf, UseColor: false,
	}
	cmd := &usageCmd{}
	err := cmd.Run(context.Background(), app, []string{"bogus"})
	if err == nil {
		t.Fatal("expected error for unknown mode")
	}
}

// TestUsage_DailyWithData verifies that daily mode reads JSONL files from a
// temp config dir (pointed to by CLAUDE_CONFIG_DIR) and outputs a non-empty
// table when entries are present.
func TestUsage_DailyWithData(t *testing.T) {
	configDir := setupUsageTempDir(t)
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)

	var stdout, stderr bytes.Buffer
	app := &App{
		Registry: &config.Registry{Main: "main", Accounts: map[string]config.Account{
			"main": {ConfigDir: configDir},
		}},
		Stdout: &stdout, Stderr: &stderr, UseColor: false,
	}
	cmd := &usageCmd{}
	// --no-cache forces a full ScanDir scan and skips the incremental cache path.
	err := cmd.Run(context.Background(), app, []string{"daily", "--no-cache"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := stdout.String()
	if out == "" {
		t.Fatal("expected non-empty output for daily mode with data")
	}
	// The output must contain a date from the sample data.
	if !strings.Contains(out, "2026-03-24") {
		t.Errorf("expected output to contain date '2026-03-24', got:\n%s", out)
	}
}

// TestUsage_JSONOutput verifies that --json produces valid JSON output.
func TestUsage_JSONOutput(t *testing.T) {
	configDir := setupUsageTempDir(t)
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)

	var stdout, stderr bytes.Buffer
	app := &App{
		Registry: &config.Registry{Main: "main", Accounts: map[string]config.Account{
			"main": {ConfigDir: configDir},
		}},
		Stdout: &stdout, Stderr: &stderr, UseColor: false,
	}
	cmd := &usageCmd{}
	err := cmd.Run(context.Background(), app, []string{"daily", "--no-cache", "--json"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := strings.TrimSpace(stdout.String())
	if out == "" {
		t.Fatal("expected non-empty JSON output")
	}
	// Verify the output is valid JSON (array of daily reports).
	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Errorf("output is not valid JSON: %v\noutput:\n%s", err, out)
	}
	if len(result) == 0 {
		t.Error("expected at least one daily report in JSON output")
	}
}

// TestUsage_UnknownFlag verifies that an unrecognised flag returns an error.
func TestUsage_UnknownFlag(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{Main: "main", Accounts: map[string]config.Account{}},
		Stdout:   &buf, Stderr: &buf, UseColor: false,
	}
	cmd := &usageCmd{}
	err := cmd.Run(context.Background(), app, []string{"daily", "--nonexistent-flag"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
	if !strings.Contains(err.Error(), "unknown flag") {
		t.Errorf("error should mention 'unknown flag', got: %v", err)
	}
}
