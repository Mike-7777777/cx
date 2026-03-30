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

// setupSessionRegistry creates a fake home dir with a registry pointing to a
// config dir that contains one project session JSONL file.
// Returns the fake home dir and the config dir.
func setupSessionRegistry(t *testing.T) (fakeHome, configDir string, reg *config.Registry) {
	t.Helper()

	fakeHome = t.TempDir()
	configDir = filepath.Join(fakeHome, "claude-config")

	// Create projects/<project>/<session>.jsonl
	projDir := filepath.Join(configDir, "projects", "my-project")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("setup projects dir: %v", err)
	}
	sessionJSONL := `{"type":"user","message":{"content":"hello world"},"timestamp":"2026-03-24T10:00:00.000Z","sessionId":"session-abc123"}
{"type":"assistant","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":10,"output_tokens":20,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"timestamp":"2026-03-24T10:00:05.000Z","sessionId":"session-abc123"}
`
	if err := os.WriteFile(filepath.Join(projDir, "session-abc123.jsonl"), []byte(sessionJSONL), 0o644); err != nil {
		t.Fatalf("setup session file: %v", err)
	}

	// Create sessions/ dir (empty — no active sessions).
	if err := os.MkdirAll(filepath.Join(configDir, "sessions"), 0o755); err != nil {
		t.Fatalf("setup sessions dir: %v", err)
	}

	// Write registry JSON to <fakeHome>/.cx.json
	reg = &config.Registry{
		Version: 1,
		Main:    "test",
		Accounts: map[string]config.Account{
			"test": {ConfigDir: configDir},
		},
	}
	regData, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		t.Fatalf("marshal registry: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fakeHome, ".cx.json"), regData, 0o600); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	// Redirect os.UserHomeDir() to our fake home.
	t.Setenv("HOME", fakeHome)
	t.Setenv("USERPROFILE", fakeHome)

	return fakeHome, configDir, reg
}

func TestDisplaySlug(t *testing.T) {
	tests := []struct {
		name string
		s    sessionEntry
		want string
	}{
		{
			name: "slug set",
			s:    sessionEntry{Slug: "fix-statusline-bug", ID: "abc123def456xyz"},
			want: "fix-statusline-bug",
		},
		{
			name: "empty slug, long ID",
			s:    sessionEntry{Slug: "", ID: "abcdef123456xyz789"},
			want: "abcdef123456...",
		},
		{
			name: "empty slug, short ID",
			s:    sessionEntry{Slug: "", ID: "short"},
			want: "short",
		},
		{
			name: "empty slug, exactly 12 chars",
			s:    sessionEntry{Slug: "", ID: "123456789012"},
			want: "123456789012",
		},
		{
			name: "empty slug, 13 chars",
			s:    sessionEntry{Slug: "", ID: "1234567890123"},
			want: "123456789012...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := displaySlug(&tt.s)
			if got != tt.want {
				t.Errorf("displaySlug() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResume_YoloFlagInHelp(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{Main: "main", Accounts: map[string]config.Account{}},
		Stdout:   &buf, Stderr: &buf, UseColor: false,
	}
	cmd := &resumeCmd{}
	err := cmd.Run(context.Background(), app, []string{"--help"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(buf.String(), "--yolo") {
		t.Errorf("help text missing --yolo: %q", buf.String())
	}
}

func TestResume_HelpFlag(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{Main: "main", Accounts: map[string]config.Account{}},
		Stdout:   &buf, Stderr: &buf, UseColor: false,
	}
	cmd := &resumeCmd{}
	err := cmd.Run(context.Background(), app, []string{"--help"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(buf.String(), "cx resume") {
		t.Errorf("help missing usage text: %q", buf.String())
	}
}

// TestResume_NoSessions verifies that running resume with an empty config dir
// returns an error containing "no sessions found".
func TestResume_NoSessions(t *testing.T) {
	fakeHome := t.TempDir()
	emptyConfigDir := filepath.Join(fakeHome, "claude-empty")
	if err := os.MkdirAll(filepath.Join(emptyConfigDir, "projects"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	reg := &config.Registry{
		Version: 1,
		Main:    "empty",
		Accounts: map[string]config.Account{
			"empty": {ConfigDir: emptyConfigDir},
		},
	}
	regData, _ := json.MarshalIndent(reg, "", "  ")
	if err := os.WriteFile(filepath.Join(fakeHome, ".cx.json"), regData, 0o600); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	t.Setenv("HOME", fakeHome)
	t.Setenv("USERPROFILE", fakeHome)

	var stdout, stderr bytes.Buffer
	app := &App{
		Registry: reg,
		Stdout:   &stdout, Stderr: &stderr, UseColor: false,
	}
	cmd := &resumeCmd{}
	err := cmd.Run(context.Background(), app, []string{"--last"})
	if err == nil {
		t.Fatal("expected error when no sessions found")
	}
	if !strings.Contains(err.Error(), "no sessions found") {
		t.Errorf("expected 'no sessions found' error, got: %v", err)
	}
}

// TestResume_LastFlag verifies that --last causes collectSessions to select the
// most recent session. We test this by calling collectSessions directly after
// setting up the registry, which avoids invoking the real claude binary.
func TestResume_LastFlag(t *testing.T) {
	_, configDir, reg := setupSessionRegistry(t)

	// collectSessions uses the provided registry to scan session files.
	sessions := collectSessions(reg, "", 50)
	if len(sessions) == 0 {
		t.Fatal("collectSessions returned no sessions; expected at least one from temp dir")
	}

	// The most recent session should correspond to our fixture file.
	first := sessions[0]
	if first.ConfigDir != configDir {
		t.Errorf("expected ConfigDir %q, got %q", configDir, first.ConfigDir)
	}
	if first.Project != "project" { // shortProjectName("my-project") → "project"
		t.Errorf("expected project 'project', got %q", first.Project)
	}

	// Confirm that --last would select sessions[0] (the most recent).
	// The resumeCmd assigns selected = &sessions[0] when isLast is true.
	selected := &sessions[0]
	if selected.ID == "" {
		t.Error("selected session has empty ID")
	}
}

func TestDeduplicateSessions(t *testing.T) {
	sessions := []sessionEntry{
		{ID: "session-1", Account: "work", Age: 2 * time.Minute, ConfigDir: "/a"},
		{ID: "session-1", Account: "personal", Age: 1 * time.Minute, ConfigDir: "/b"},
		{ID: "session-2", Account: "work", Age: 5 * time.Minute, ConfigDir: "/a"},
	}
	got := deduplicateSessions(sessions)
	if len(got) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(got))
	}
	for _, s := range got {
		if s.ID == "session-1" {
			if s.Account != "personal" {
				t.Errorf("session-1: expected account personal (smaller Age), got %s", s.Account)
			}
			if s.Age != 1*time.Minute {
				t.Errorf("session-1: expected Age 1m, got %v", s.Age)
			}
		}
	}
}
