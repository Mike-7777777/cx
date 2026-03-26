package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Mike-7777777/cx/internal/config"
)

func TestLogin_InvalidName(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{Main: "main", Accounts: map[string]config.Account{
			"main": {ConfigDir: t.TempDir()},
		}},
		Stdout: &buf, Stderr: &buf, UseColor: false,
	}
	cmd := &loginCmd{}
	// An account name with path separators should be rejected by the name validator.
	err := cmd.Run(context.Background(), app, []string{"../evil"})
	if err == nil {
		t.Fatal("expected error for invalid account name")
	}
}

func TestLogin_Help(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{Main: "main", Accounts: map[string]config.Account{}},
		Stdout:   &buf, Stderr: &buf, UseColor: false,
	}
	cmd := &loginCmd{}
	err := cmd.Run(context.Background(), app, []string{"--help"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(buf.String(), "cx login") {
		t.Errorf("help missing usage text: %q", buf.String())
	}
}
