package main

import (
	"bytes"
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/Mike-7777777/cx/internal/config"
)

// testConfigDir returns a safe absolute path usable on the current OS.
func testConfigDir(name string) string {
	if runtime.GOOS == "windows" {
		return "C:\\Users\\testuser\\" + name
	}
	return "/home/user/" + name
}

func TestSwitch_BashOutput(t *testing.T) {
	mainDir := testConfigDir(".claude")
	altDir := testConfigDir(".claude-alt")

	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{
			Main: "main",
			Accounts: map[string]config.Account{
				"main": {ConfigDir: mainDir},
				"alt":  {ConfigDir: altDir},
			},
		},
		Stdout: &buf, Stderr: &buf, UseColor: false,
	}
	cmd := &switchCmd{}
	err := cmd.Run(context.Background(), app, []string{"alt", "--shell=bash", "--no-sync"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "export CLAUDE_CONFIG_DIR=") {
		t.Errorf("missing export in output: %q", out)
	}
	if !strings.Contains(out, altDir) {
		t.Errorf("missing config dir path in output: %q", out)
	}
}

func TestSwitch_MainUnsetsEnv(t *testing.T) {
	mainDir := testConfigDir(".claude")

	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{
			Main: "main",
			Accounts: map[string]config.Account{
				"main": {ConfigDir: mainDir},
			},
		},
		Stdout: &buf, Stderr: &buf, UseColor: false,
	}
	cmd := &switchCmd{}
	err := cmd.Run(context.Background(), app, []string{"main", "--shell=bash", "--no-sync"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(buf.String(), "unset CLAUDE_CONFIG_DIR") {
		t.Errorf("missing unset for main account: %q", buf.String())
	}
}

func TestSwitch_UnsafePathRejected(t *testing.T) {
	unsafePath := "/tmp/$(rm -rf /)"
	if runtime.GOOS == "windows" {
		unsafePath = "C:\\tmp\\$(rm -rf /)"
	}
	app := &App{
		Registry: &config.Registry{
			Main: "main",
			Accounts: map[string]config.Account{
				"evil": {ConfigDir: unsafePath},
			},
		},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, UseColor: false,
	}
	cmd := &switchCmd{}
	err := cmd.Run(context.Background(), app, []string{"evil", "--shell=bash", "--no-sync"})
	if err == nil {
		t.Fatal("expected error for unsafe path")
	}
}

func TestIsSafePath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/home/user/.config", true},
		{"C:\\Users\\test", true},
		{"/tmp/a-b_c.d", true},
		{"~/bin", true},
		{"C:/Program Files/cx", true},
		{"", false},
		{"/tmp/$(rm -rf /)", false},
		{"/tmp/`whoami`", false},
		{"/tmp/a;b", false},
		{"/tmp/a|b", false},
		{"/tmp/a&b", false},
		{"/tmp/a>b", false},
	}
	for _, tt := range tests {
		if got := isSafePath(tt.path); got != tt.want {
			t.Errorf("isSafePath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestSwitch_UnknownAccount(t *testing.T) {
	mainDir := testConfigDir(".claude")

	app := &App{
		Registry: &config.Registry{
			Main: "main",
			Accounts: map[string]config.Account{
				"main": {ConfigDir: mainDir},
			},
		},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, UseColor: false,
	}
	cmd := &switchCmd{}
	err := cmd.Run(context.Background(), app, []string{"nonexistent", "--shell=bash", "--no-sync"})
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestSwitch_InvalidName(t *testing.T) {
	app := &App{
		Registry: &config.Registry{
			Main:     "main",
			Accounts: map[string]config.Account{},
		},
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, UseColor: false,
	}
	cmd := &switchCmd{}
	err := cmd.Run(context.Background(), app, []string{"../etc/passwd", "--shell=bash", "--no-sync"})
	if err == nil {
		t.Fatal("expected error for invalid name")
	}
}
