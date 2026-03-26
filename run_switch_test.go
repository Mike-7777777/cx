package main

import (
	"bytes"
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/platform"
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
	cmd := &switchCmd{shell: platform.ShellBash, noSync: true}
	err := cmd.Run(context.Background(), app, []string{"alt"})
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
	cmd := &switchCmd{shell: platform.ShellBash, noSync: true}
	err := cmd.Run(context.Background(), app, []string{"main"})
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
	cmd := &switchCmd{shell: platform.ShellBash, noSync: true}
	err := cmd.Run(context.Background(), app, []string{"evil"})
	if err == nil {
		t.Fatal("expected error for unsafe path")
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
	cmd := &switchCmd{shell: platform.ShellBash, noSync: true}
	err := cmd.Run(context.Background(), app, []string{"nonexistent"})
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
	cmd := &switchCmd{shell: platform.ShellBash, noSync: true}
	err := cmd.Run(context.Background(), app, []string{"../etc/passwd"})
	if err == nil {
		t.Fatal("expected error for invalid name")
	}
}
