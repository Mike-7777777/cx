package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
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

func TestParseResumeArgs_Empty(t *testing.T) {
	opts, err := parseResumeArgs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if opts.showHelp || opts.last || opts.on != "" || opts.prefer != "" ||
		opts.searchTerm != "" || len(opts.claudeArgs) != 0 {
		t.Errorf("empty args should be zero-valued: %+v", opts)
	}
}

func TestParseResumeArgs_Help(t *testing.T) {
	for _, a := range []string{"--help", "-h"} {
		opts, err := parseResumeArgs([]string{a})
		if err != nil {
			t.Fatal(err)
		}
		if !opts.showHelp {
			t.Errorf("%s should set showHelp", a)
		}
	}
}

func TestParseResumeArgs_Last(t *testing.T) {
	opts, err := parseResumeArgs([]string{"--last"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.last {
		t.Error("--last should set last=true")
	}
}

func TestParseResumeArgs_On(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--on", "work"}, "work"},
		{[]string{"--on=work"}, "work"},
	}
	for _, tc := range cases {
		opts, err := parseResumeArgs(tc.args)
		if err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		if opts.on != tc.want {
			t.Errorf("%v: on=%q, want %q", tc.args, opts.on, tc.want)
		}
	}
}

func TestParseResumeArgs_OnMissingValue(t *testing.T) {
	for _, args := range [][]string{{"--on"}, {"--on="}} {
		if _, err := parseResumeArgs(args); err == nil {
			t.Errorf("%v: expected error", args)
		}
	}
}

func TestParseResumeArgs_Prefer(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--prefer", "QM"}, "QM"},
		{[]string{"--prefer=QM"}, "QM"},
		{[]string{"-pf", "QM"}, "QM"},
	}
	for _, tc := range cases {
		opts, err := parseResumeArgs(tc.args)
		if err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		if opts.prefer != tc.want {
			t.Errorf("%v: prefer=%q, want %q", tc.args, opts.prefer, tc.want)
		}
	}
}

func TestParseResumeArgs_PreferMissingValue(t *testing.T) {
	for _, args := range [][]string{{"--prefer"}, {"-pf"}, {"--prefer="}} {
		if _, err := parseResumeArgs(args); err == nil {
			t.Errorf("%v: expected error", args)
		}
	}
}

func TestParseResumeArgs_OnAndPreferMutuallyExclusive(t *testing.T) {
	_, err := parseResumeArgs([]string{"--on", "a", "--prefer", "b"})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected mutually-exclusive error, got %v", err)
	}
}

func TestParseResumeArgs_YoloExpandsToDangerousFlag(t *testing.T) {
	for _, a := range []string{"-y", "--yolo"} {
		opts, err := parseResumeArgs([]string{a})
		if err != nil {
			t.Fatal(err)
		}
		if len(opts.claudeArgs) != 1 || opts.claudeArgs[0] != "--dangerously-skip-permissions" {
			t.Errorf("%s: claudeArgs=%v, want [--dangerously-skip-permissions]", a, opts.claudeArgs)
		}
	}
}

func TestParseResumeArgs_SearchTerm(t *testing.T) {
	opts, err := parseResumeArgs([]string{"fix-bug"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.searchTerm != "fix-bug" {
		t.Errorf("searchTerm=%q, want fix-bug", opts.searchTerm)
	}
}

func TestParseResumeArgs_UnknownFlagsPassToClaude(t *testing.T) {
	// --model sonnet survives intact because the search-term-first rule means
	// bare words after a flag are not treated as search terms.
	opts, err := parseResumeArgs([]string{"--remote-control", "-rc", "--model", "sonnet"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.searchTerm != "" {
		t.Errorf("searchTerm=%q, want empty (no leading positional)", opts.searchTerm)
	}
	want := []string{"--remote-control", "-rc", "--model", "sonnet"}
	if len(opts.claudeArgs) != len(want) {
		t.Fatalf("claudeArgs=%v, want %v", opts.claudeArgs, want)
	}
	for i, v := range want {
		if opts.claudeArgs[i] != v {
			t.Errorf("claudeArgs[%d]=%q, want %q", i, opts.claudeArgs[i], v)
		}
	}
}

// TestParseResumeArgs_TermFirstThenClaudeFlagValue verifies that a leading
// search term plus a value-bearing claude flag (--model sonnet) parses
// unambiguously: term=fix-bug, claudeArgs=[--model, sonnet].
func TestParseResumeArgs_TermFirstThenClaudeFlagValue(t *testing.T) {
	opts, err := parseResumeArgs([]string{"fix-bug", "--model", "sonnet"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.searchTerm != "fix-bug" {
		t.Errorf("searchTerm=%q, want fix-bug", opts.searchTerm)
	}
	want := []string{"--model", "sonnet"}
	if len(opts.claudeArgs) != len(want) {
		t.Fatalf("claudeArgs=%v, want %v", opts.claudeArgs, want)
	}
	for i, v := range want {
		if opts.claudeArgs[i] != v {
			t.Errorf("claudeArgs[%d]=%q, want %q", i, opts.claudeArgs[i], v)
		}
	}
}

// TestParseResumeArgs_DoubleDashSeparator verifies explicit passthrough.
func TestParseResumeArgs_DoubleDashSeparator(t *testing.T) {
	opts, err := parseResumeArgs([]string{"--last", "--", "-p", "continue"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.last {
		t.Error("--last before -- should still be consumed")
	}
	if len(opts.claudeArgs) != 2 || opts.claudeArgs[0] != "-p" || opts.claudeArgs[1] != "continue" {
		t.Errorf("claudeArgs=%v, want [-p continue]", opts.claudeArgs)
	}
}

// TestParseResumeArgs_Regression_RcYoloPreferQM is the exact user bug report:
// `cx resume --rc --yolo --prefer QM` used to fail with "no session matching
// --rc". It must now parse as prefer=QM, claudeArgs=[--rc, --dangerously-skip-permissions].
func TestParseResumeArgs_Regression_RcYoloPreferQM(t *testing.T) {
	opts, err := parseResumeArgs([]string{"--rc", "--yolo", "--prefer", "QM"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.prefer != "QM" {
		t.Errorf("prefer=%q, want QM", opts.prefer)
	}
	if opts.searchTerm != "" {
		t.Errorf("searchTerm=%q, want empty (not a search term)", opts.searchTerm)
	}
	want := []string{"--rc", "--dangerously-skip-permissions"}
	if len(opts.claudeArgs) != len(want) {
		t.Fatalf("claudeArgs=%v, want %v", opts.claudeArgs, want)
	}
	for i, v := range want {
		if opts.claudeArgs[i] != v {
			t.Errorf("claudeArgs[%d]=%q, want %q", i, opts.claudeArgs[i], v)
		}
	}
}

// TestParseResumeArgs_Regression_DashRc covers the `-rc` single-dash variant
// (user's second attempt). Should also pass through unchanged.
func TestParseResumeArgs_Regression_DashRc(t *testing.T) {
	opts, err := parseResumeArgs([]string{"-rc", "--yolo", "--prefer", "QM"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.prefer != "QM" {
		t.Errorf("prefer=%q, want QM", opts.prefer)
	}
	if opts.searchTerm != "" {
		t.Errorf("searchTerm=%q, want empty", opts.searchTerm)
	}
	// -rc goes to claudeArgs because it starts with - and is not in runAliases
	// and is not known to cx.
	if !slices.Contains(opts.claudeArgs, "-rc") {
		t.Errorf("expected -rc in claudeArgs, got %v", opts.claudeArgs)
	}
}

func TestParseResumeArgs_TermAndFlagsTogether(t *testing.T) {
	opts, err := parseResumeArgs([]string{"fix-bug", "--prefer", "QM", "--remote-control"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.searchTerm != "fix-bug" {
		t.Errorf("searchTerm=%q, want fix-bug", opts.searchTerm)
	}
	if opts.prefer != "QM" {
		t.Errorf("prefer=%q, want QM", opts.prefer)
	}
	if len(opts.claudeArgs) != 1 || opts.claudeArgs[0] != "--remote-control" {
		t.Errorf("claudeArgs=%v, want [--remote-control]", opts.claudeArgs)
	}
}

func TestDeduplicateSessions(t *testing.T) {
	sessions := []sessionEntry{
		{ID: "session-1", Account: "acct1", Age: 2 * time.Minute, ConfigDir: "/a"},
		{ID: "session-1", Account: "acct2", Age: 1 * time.Minute, ConfigDir: "/b"},
		{ID: "session-2", Account: "acct1", Age: 5 * time.Minute, ConfigDir: "/a"},
	}
	got := deduplicateSessions(sessions)
	if len(got) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(got))
	}
	for _, s := range got {
		if s.ID == "session-1" {
			if s.Account != "acct2" {
				t.Errorf("session-1: expected account acct2 (smaller Age), got %s", s.Account)
			}
			if s.Age != 1*time.Minute {
				t.Errorf("session-1: expected Age 1m, got %v", s.Age)
			}
		}
	}
}
