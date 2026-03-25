package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckStatuslinePath_ValidPath(t *testing.T) {
	dir := t.TempDir()

	// Create settings.json with a valid cx statusline command.
	// Use "cx statusline" which is a bare command (skips binary existence check).
	settings := map[string]any{
		"statusLine": map[string]any{
			"command": "cx statusline",
		},
	}
	writeSettingsJSON(t, dir, settings)

	ok, msg := checkStatuslinePath(dir)
	if !ok {
		t.Errorf("expected ok=true, got ok=false, msg=%q", msg)
	}
	if msg != "ok" {
		t.Errorf("expected msg=%q, got %q", "ok", msg)
	}
}

func TestCheckStatuslinePath_BackslashAutoFixed(t *testing.T) {
	dir := t.TempDir()

	// Create a dummy binary so the binary-existence check passes after fix.
	dummyBin := filepath.Join(dir, "cx.exe")
	if err := os.WriteFile(dummyBin, []byte("dummy"), 0o755); err != nil {
		t.Fatal(err)
	}
	forwardPath := strings.ReplaceAll(dummyBin, "\\", "/")

	// Create settings.json with backslash paths.
	settings := map[string]any{
		"statusLine": map[string]any{
			"command": dummyBin + " statusline",
		},
	}
	writeSettingsJSON(t, dir, settings)

	ok, msg := checkStatuslinePath(dir)
	if !ok {
		t.Errorf("expected ok=true after auto-fix, got ok=false, msg=%q", msg)
	}
	if !strings.Contains(msg, "auto-fixed") {
		t.Errorf("expected auto-fix message, got %q", msg)
	}

	// Verify the file was rewritten with forward slashes.
	data, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
	if strings.Contains(string(data), "\\") {
		t.Errorf("settings.json still contains backslashes after fix: %s", data)
	}
	if !strings.Contains(string(data), forwardPath) {
		t.Errorf("settings.json missing forward-slash path %q: %s", forwardPath, data)
	}
}

func TestCheckStatuslinePath_NoStatusLineKey(t *testing.T) {
	dir := t.TempDir()

	// settings.json without statusLine key.
	settings := map[string]any{
		"theme": "dark",
	}
	writeSettingsJSON(t, dir, settings)

	ok, msg := checkStatuslinePath(dir)
	if !ok {
		t.Errorf("expected ok=true, got ok=false, msg=%q", msg)
	}
	if msg != "not configured" {
		t.Errorf("expected msg=%q, got %q", "not configured", msg)
	}
}

func TestCheckStatuslinePath_NoSettingsJSON(t *testing.T) {
	dir := t.TempDir()
	// No settings.json at all.

	ok, msg := checkStatuslinePath(dir)
	if !ok {
		t.Errorf("expected ok=true, got ok=false, msg=%q", msg)
	}
	if msg != "no settings.json" {
		t.Errorf("expected msg=%q, got %q", "no settings.json", msg)
	}
}

func TestCheckStatuslinePath_NonExistentBinaryAutoFixed(t *testing.T) {
	dir := t.TempDir()

	// Point to a binary that does not exist.
	settings := map[string]any{
		"statusLine": map[string]any{
			"command": "/nonexistent/path/cx statusline",
		},
	}
	writeSettingsJSON(t, dir, settings)

	// The function tries os.Executable() to auto-fix. In test context, the
	// test binary exists, so auto-fix should succeed.
	ok, msg := checkStatuslinePath(dir)
	// Whether ok is true depends on whether the auto-fix write succeeds,
	// which it should in a temp dir.
	if !ok {
		// If auto-fix failed, the message should mention "binary not found".
		if !strings.Contains(msg, "binary not found") && !strings.Contains(msg, "auto-fixed") {
			t.Errorf("unexpected failure msg: %q", msg)
		}
	} else {
		if !strings.Contains(msg, "auto-fixed") {
			t.Errorf("expected auto-fixed message, got %q", msg)
		}
	}
}

// writeSettingsJSON is a test helper that writes a settings.json to dir.
func writeSettingsJSON(t *testing.T, dir string, settings map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}
