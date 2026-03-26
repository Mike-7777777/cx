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

// TestSync_NoAccounts verifies that sync succeeds with no secondary accounts.
func TestSync_NoAccounts(t *testing.T) {
	mainDir := t.TempDir()

	var stderr bytes.Buffer
	app := &App{
		Registry: &config.Registry{
			Main: "main",
			Accounts: map[string]config.Account{
				"main": {ConfigDir: mainDir},
			},
		},
		Stdout: &bytes.Buffer{}, Stderr: &stderr, UseColor: false,
	}

	cmd := &syncCmd{}
	if err := cmd.Run(context.Background(), app, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// No output expected — nothing to sync.
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr output: %q", stderr.String())
	}
}

// TestSync_SkipsMainAccount verifies that the main account is not synced to itself.
func TestSync_SkipsMainAccount(t *testing.T) {
	mainDir := t.TempDir()

	// Write a settings.json in mainDir.
	settingsContent := []byte(`{"theme":"dark"}`)
	if err := os.WriteFile(filepath.Join(mainDir, "settings.json"), settingsContent, 0o600); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	app := &App{
		Registry: &config.Registry{
			Main: "main",
			Accounts: map[string]config.Account{
				"main": {ConfigDir: mainDir},
			},
		},
		Stdout: &bytes.Buffer{}, Stderr: &stderr, UseColor: false,
	}

	cmd := &syncCmd{}
	if err := cmd.Run(context.Background(), app, []string{"--force"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// No "syncing" lines should appear because no secondary accounts exist.
	if strings.Contains(stderr.String(), "syncing") {
		t.Errorf("sync should not write output for main-only registry: %q", stderr.String())
	}
}

// TestSync_CopiesSettingsJSON verifies that settings.json is copied from main to secondary.
func TestSync_CopiesSettingsJSON(t *testing.T) {
	mainDir := t.TempDir()
	secondaryDir := t.TempDir()

	settingsContent := []byte(`{"theme":"dark"}`)
	if err := os.WriteFile(filepath.Join(mainDir, "settings.json"), settingsContent, 0o600); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	app := &App{
		Registry: &config.Registry{
			Main: "main",
			Accounts: map[string]config.Account{
				"main": {ConfigDir: mainDir},
				"alt":  {ConfigDir: secondaryDir},
			},
		},
		Stdout: &bytes.Buffer{}, Stderr: &stderr, UseColor: false,
	}

	cmd := &syncCmd{}
	if err := cmd.Run(context.Background(), app, []string{"--force"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	dst := filepath.Join(secondaryDir, "settings.json")
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("settings.json not written to secondary: %v", err)
	}
	if !bytes.Equal(got, settingsContent) {
		t.Errorf("settings.json content mismatch: got %q, want %q", got, settingsContent)
	}
}

// TestSync_CopiesPluginFiles verifies that plugin config files are copied from main to secondary.
func TestSync_CopiesPluginFiles(t *testing.T) {
	mainDir := t.TempDir()
	secondaryDir := t.TempDir()

	pluginsDir := filepath.Join(mainDir, "plugins")
	if err := os.MkdirAll(pluginsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	pluginContent := []byte(`{"plugins":[]}`)
	if err := os.WriteFile(filepath.Join(pluginsDir, "installed_plugins.json"), pluginContent, 0o600); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	app := &App{
		Registry: &config.Registry{
			Main: "main",
			Accounts: map[string]config.Account{
				"main": {ConfigDir: mainDir},
				"alt":  {ConfigDir: secondaryDir},
			},
		},
		Stdout: &bytes.Buffer{}, Stderr: &stderr, UseColor: false,
	}

	cmd := &syncCmd{}
	if err := cmd.Run(context.Background(), app, []string{"--force"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	dst := filepath.Join(secondaryDir, "plugins", "installed_plugins.json")
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("installed_plugins.json not written to secondary: %v", err)
	}
	if !bytes.Equal(got, pluginContent) {
		t.Errorf("installed_plugins.json content mismatch: got %q, want %q", got, pluginContent)
	}
}

// TestSync_SkipsAccountWithEmptyDir verifies that accounts with no ConfigDir are skipped.
func TestSync_SkipsAccountWithEmptyDir(t *testing.T) {
	mainDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(mainDir, "settings.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	app := &App{
		Registry: &config.Registry{
			Main: "main",
			Accounts: map[string]config.Account{
				"main":   {ConfigDir: mainDir},
				"no-dir": {ConfigDir: ""},
			},
		},
		Stdout: &bytes.Buffer{}, Stderr: &stderr, UseColor: false,
	}

	cmd := &syncCmd{}
	// Should succeed without attempting to write to the empty-dir account.
	if err := cmd.Run(context.Background(), app, []string{"--force"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// "syncing" should not appear for the empty-dir account.
	if strings.Contains(stderr.String(), `"no-dir"`) {
		t.Errorf("sync should skip account with empty ConfigDir, got: %q", stderr.String())
	}
}

// TestSync_ForceFlag verifies that --force is parsed and sync proceeds without prompting.
func TestSync_ForceFlag(t *testing.T) {
	mainDir := t.TempDir()
	secondaryDir := t.TempDir()

	content := []byte(`{"model":"claude-3"}`)
	if err := os.WriteFile(filepath.Join(mainDir, "settings.json"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	// Write a secondary settings.json that is explicitly "older" by writing it first,
	// then rewriting main — on most filesystems mod-time resolution is coarse enough
	// that --force is the only reliable way to test overwrite-without-prompt.
	if err := os.WriteFile(filepath.Join(secondaryDir, "settings.json"), []byte(`{"model":"old"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	app := &App{
		Registry: &config.Registry{
			Main: "main",
			Accounts: map[string]config.Account{
				"main": {ConfigDir: mainDir},
				"alt":  {ConfigDir: secondaryDir},
			},
		},
		Stdout: &bytes.Buffer{}, Stderr: &stderr, UseColor: false,
	}

	cmd := &syncCmd{}
	if err := cmd.Run(context.Background(), app, []string{"--force"}); err != nil {
		t.Fatalf("Run with --force: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(secondaryDir, "settings.json"))
	if err != nil {
		t.Fatalf("reading secondary settings.json: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("--force did not overwrite secondary settings.json: got %q, want %q", got, content)
	}
}

// TestSync_UnknownMainAccount verifies that an error is returned when the main
// account is not present in the registry.
func TestSync_UnknownMainAccount(t *testing.T) {
	app := &App{
		Registry: &config.Registry{
			Main: "main",
			Accounts: map[string]config.Account{
				"other": {ConfigDir: t.TempDir()},
			},
		},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, UseColor: false,
	}

	cmd := &syncCmd{}
	err := cmd.Run(context.Background(), app, []string{"--force"})
	if err == nil {
		t.Fatal("expected error when main account is not in registry")
	}
}

// TestSync_MultipleSecondaries verifies that all secondary accounts receive the synced files.
func TestSync_MultipleSecondaries(t *testing.T) {
	mainDir := t.TempDir()
	secondaryA := t.TempDir()
	secondaryB := t.TempDir()

	content := []byte(`{"key":"value"}`)
	if err := os.WriteFile(filepath.Join(mainDir, "settings.json"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	app := &App{
		Registry: &config.Registry{
			Main: "main",
			Accounts: map[string]config.Account{
				"main":  {ConfigDir: mainDir},
				"acc-a": {ConfigDir: secondaryA},
				"acc-b": {ConfigDir: secondaryB},
			},
		},
		Stdout: &bytes.Buffer{}, Stderr: &stderr, UseColor: false,
	}

	cmd := &syncCmd{}
	if err := cmd.Run(context.Background(), app, []string{"--force"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, dir := range []string{secondaryA, secondaryB} {
		got, err := os.ReadFile(filepath.Join(dir, "settings.json"))
		if err != nil {
			t.Errorf("settings.json not found in %s: %v", dir, err)
			continue
		}
		if !bytes.Equal(got, content) {
			t.Errorf("settings.json mismatch in %s: got %q, want %q", dir, got, content)
		}
	}
}

// TestSync_MissingSourceFilesSkipped verifies that syncFileList entries missing
// in the main account are silently skipped — no error, no destination file created.
func TestSync_MissingSourceFilesSkipped(t *testing.T) {
	mainDir := t.TempDir()
	secondaryDir := t.TempDir()

	// Do NOT create any files in mainDir — all syncFileList entries are absent.

	var stderr bytes.Buffer
	app := &App{
		Registry: &config.Registry{
			Main: "main",
			Accounts: map[string]config.Account{
				"main": {ConfigDir: mainDir},
				"alt":  {ConfigDir: secondaryDir},
			},
		},
		Stdout: &bytes.Buffer{}, Stderr: &stderr, UseColor: false,
	}

	cmd := &syncCmd{}
	if err := cmd.Run(context.Background(), app, []string{"--force"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// No files should have been created in secondaryDir.
	entries, err := os.ReadDir(secondaryDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("secondaryDir should be empty but contains: %v", names)
	}
}
