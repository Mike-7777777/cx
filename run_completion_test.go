package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Mike-7777777/cx/internal/config"
)

func TestCompletion_Bash(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{Main: "main", Accounts: map[string]config.Account{
			"main": {}, "alt": {},
		}},
		Stdout: &buf, Stderr: &buf, UseColor: false,
	}
	cmd := &completionCmd{}
	err := cmd.Run(context.Background(), app, []string{"bash"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "complete") || !strings.Contains(out, "cx") {
		t.Errorf("bash completion missing expected content: %q", out[:min(len(out), 200)])
	}
}

func TestCompletion_NoArgs(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{Main: "main", Accounts: map[string]config.Account{}},
		Stdout:   &buf, Stderr: &buf, UseColor: false,
	}
	cmd := &completionCmd{}
	err := cmd.Run(context.Background(), app, nil)
	if err == nil {
		t.Fatal("expected error when no shell specified")
	}
}
