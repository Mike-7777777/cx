package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/Mike-7777777/cx/internal/config"
)

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
