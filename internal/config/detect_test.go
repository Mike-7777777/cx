package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectConfigDir_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)

	got, err := DetectConfigDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	abs, _ := filepath.Abs(dir)
	if got != abs {
		t.Errorf("got %q, want %q", got, abs)
	}
}

func TestDetectConfigDir_FallbackDotClaude(t *testing.T) {
	// Unset env overrides so fallback logic runs.
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	// Create a fake home with a .claude directory.
	fakeHome := t.TempDir()
	dotClaude := filepath.Join(fakeHome, ".claude")
	if err := os.MkdirAll(dotClaude, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Override home directory via environment variable used by os.UserHomeDir on
	// all platforms (HOME on Unix, USERPROFILE on Windows).
	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")
	os.Setenv("HOME", fakeHome)
	os.Setenv("USERPROFILE", fakeHome)
	t.Cleanup(func() {
		os.Setenv("HOME", origHome)
		os.Setenv("USERPROFILE", origUserProfile)
	})

	got, err := DetectConfigDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != dotClaude {
		t.Errorf("got %q, want %q", got, dotClaude)
	}
}
