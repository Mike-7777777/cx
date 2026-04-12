package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mike-7777777/cx/internal/config"
)

func TestShortProjectName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"D--projects-myapp", "myapp"},
		{"C--Users-foo-project", "project"},
		{"single", "single"},
		{"a-b-c", "c"},
		{"", ""},
	}

	for _, tt := range tests {
		got := shortProjectName(tt.input)
		if got != tt.want {
			t.Errorf("shortProjectName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  string
	}{
		{30 * time.Second, "now"},
		{5 * time.Minute, "5m ago"},
		{3 * time.Hour, "3h ago"},
		{48 * time.Hour, "2d ago"},
		{0, "now"},
		{59 * time.Second, "now"},
		{90 * time.Minute, "1h ago"},
		{25 * time.Hour, "1d ago"},
	}

	for _, tt := range tests {
		got := formatAge(tt.input)
		if got != tt.want {
			t.Errorf("formatAge(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// setupSessionsRegistry creates a fake home with registry + session JSONL file
// for testing sessionsCmd. Returns the fake home and config dir.
func setupSessionsRegistry(t *testing.T) (fakeHome, configDir string) {
	t.Helper()

	fakeHome = t.TempDir()
	configDir = filepath.Join(fakeHome, "claude-config")

	projDir := filepath.Join(configDir, "projects", "test-project")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("setup projects dir: %v", err)
	}
	sessionJSONL := `{"type":"user","message":{"content":"test question"},"timestamp":"2026-03-24T10:00:00.000Z","sessionId":"sess-001"}
{"type":"assistant","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":50,"output_tokens":100,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"timestamp":"2026-03-24T10:00:05.000Z","sessionId":"sess-001"}
`
	if err := os.WriteFile(filepath.Join(projDir, "sess-001.jsonl"), []byte(sessionJSONL), 0o644); err != nil {
		t.Fatalf("setup session JSONL: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(configDir, "sessions"), 0o755); err != nil {
		t.Fatalf("setup sessions dir: %v", err)
	}

	reg := &config.Registry{
		Version: 1,
		Main:    "test",
		Accounts: map[string]config.Account{
			"test": {ConfigDir: configDir},
		},
	}
	regData, _ := json.MarshalIndent(reg, "", "  ")
	if err := os.WriteFile(filepath.Join(fakeHome, ".cx.json"), regData, 0o600); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	t.Setenv("HOME", fakeHome)
	t.Setenv("USERPROFILE", fakeHome)

	return fakeHome, configDir
}

// TestSessions_JSONOutput verifies that --json produces a valid JSON array of sessions.
func TestSessions_JSONOutput(t *testing.T) {
	_, configDir := setupSessionsRegistry(t)

	var stdout, stderr bytes.Buffer
	app := &App{
		Registry: &config.Registry{
			Version: 1,
			Main:    "test",
			Accounts: map[string]config.Account{
				"test": {ConfigDir: configDir},
			},
		},
		Stdout: &stdout, Stderr: &stderr, UseColor: false,
	}
	cmd := &sessionsCmd{}
	err := cmd.Run(context.Background(), app, []string{"--json"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := strings.TrimSpace(stdout.String())
	if out == "" || out == "No sessions found." {
		t.Fatalf("expected JSON output, got: %q", out)
	}
	var sessions []map[string]any
	if err := json.Unmarshal([]byte(out), &sessions); err != nil {
		t.Errorf("output is not valid JSON: %v\noutput:\n%s", err, out)
	}
	if len(sessions) == 0 {
		t.Error("expected at least one session in JSON output")
	}
	// Each session entry must have an "id" field.
	if _, ok := sessions[0]["id"]; !ok {
		t.Errorf("session entry missing 'id' field: %v", sessions[0])
	}
}

// TestSessions_Help verifies that --help produces usage text on stdout.
func TestSessions_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	app := &App{
		Registry: &config.Registry{Main: "main", Accounts: map[string]config.Account{}},
		Stdout:   &stdout, Stderr: &stderr, UseColor: false,
	}
	cmd := &sessionsCmd{}
	err := cmd.Run(context.Background(), app, []string{"--help"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "cx sessions") {
		t.Errorf("help output missing 'cx sessions', got:\n%s", out)
	}
	if !strings.Contains(out, "--json") {
		t.Errorf("help output missing '--json' flag description, got:\n%s", out)
	}
}
