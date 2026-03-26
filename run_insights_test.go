package main

import (
	"bytes"
	"context"
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
