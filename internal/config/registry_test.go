package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	cxerrors "github.com/Mike-7777777/cx/internal/errors"
)

func TestRegistry_AddAndList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.json")

	r, err := LoadOrCreateRegistry(path)
	if err != nil {
		t.Fatalf("LoadOrCreateRegistry: %v", err)
	}

	r.AddAccount("work", filepath.Join(dir, "claude-work"))
	r.AddAccount("personal", filepath.Join(dir, "claude-personal"))

	if err := r.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Reload and verify persistence.
	r2, err := LoadOrCreateRegistry(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(r2.Accounts) != 2 {
		t.Errorf("got %d accounts, want 2", len(r2.Accounts))
	}
	if _, ok := r2.Accounts["work"]; !ok {
		t.Error("account 'work' missing after reload")
	}
	if _, ok := r2.Accounts["personal"]; !ok {
		t.Error("account 'personal' missing after reload")
	}
}

func TestRegistry_ResolveConfigDir(t *testing.T) {
	// Set up a real .claude directory so DetectConfigDir succeeds.
	fakeHome := t.TempDir()
	dotClaude := filepath.Join(fakeHome, ".claude")
	if err := os.MkdirAll(dotClaude, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", fakeHome)
	t.Setenv("USERPROFILE", fakeHome)

	dir := t.TempDir()
	r, err := LoadOrCreateRegistry(filepath.Join(dir, "registry.json"))
	if err != nil {
		t.Fatalf("LoadOrCreateRegistry: %v", err)
	}

	// Account with empty config_dir should fall back to DetectConfigDir.
	r.AddAccount("default", "")

	resolved, err := r.ResolveConfigDir("default")
	if err != nil {
		t.Fatalf("ResolveConfigDir: %v", err)
	}
	if resolved != dotClaude {
		t.Errorf("got %q, want %q", resolved, dotClaude)
	}

	// Account with explicit config_dir should return it as-is.
	explicit := filepath.Join(dir, "claude-work")
	r.AddAccount("work", explicit)
	resolved2, err := r.ResolveConfigDir("work")
	if err != nil {
		t.Fatalf("ResolveConfigDir(work): %v", err)
	}
	if resolved2 != explicit {
		t.Errorf("got %q, want %q", resolved2, explicit)
	}

	// Unknown account should return ErrAccountNotFound.
	_, err = r.ResolveConfigDir("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent account, got nil")
	}
	if !errors.Is(err, cxerrors.ErrAccountNotFound) {
		t.Errorf("expected ErrAccountNotFound, got: %v", err)
	}
}
