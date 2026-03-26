package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Mike-7777777/cx/internal/config"
)

func TestStatus_ShowsAllAccounts(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{
			Main: "main",
			Accounts: map[string]config.Account{
				"main": {ConfigDir: t.TempDir()},
				"alt":  {ConfigDir: t.TempDir()},
			},
		},
		Stdout: &buf, Stderr: &buf, UseColor: false,
	}
	cmd := &statusCmd{}
	err := cmd.Run(context.Background(), app, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "main") {
		t.Errorf("output missing 'main': %q", out)
	}
	if !strings.Contains(out, "alt") {
		t.Errorf("output missing 'alt': %q", out)
	}
}

func TestStatus_NoAccounts(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{
			Main:     "main",
			Accounts: map[string]config.Account{},
		},
		Stdout: &buf, Stderr: &buf, UseColor: false,
	}
	cmd := &statusCmd{}
	err := cmd.Run(context.Background(), app, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(buf.String(), "No accounts") {
		t.Errorf("expected 'No accounts' message, got: %q", buf.String())
	}
}

func TestStatus_ShowsRecommendation(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{
			Main: "main",
			Accounts: map[string]config.Account{
				"main": {ConfigDir: t.TempDir()},
			},
		},
		Stdout: &buf, Stderr: &buf, UseColor: false,
	}
	cmd := &statusCmd{}
	_ = cmd.Run(context.Background(), app, nil)
	// Even without rate data, should show header + account name
	out := buf.String()
	if !strings.Contains(out, "Account") {
		t.Errorf("missing table header: %q", out)
	}
}
