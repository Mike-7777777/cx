package main

import (
	"bytes"
	"testing"

	"github.com/Mike-7777777/cx/internal/config"
)

func newTestApp(t *testing.T) (*App, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	reg := &config.Registry{
		Main:     "main",
		Accounts: map[string]config.Account{},
	}
	app := &App{
		Registry: reg,
		Stdout:   &buf,
		Stderr:   &buf,
		UseColor: false,
	}
	return app, &buf
}

func TestNewApp(t *testing.T) {
	app, buf := newTestApp(t)
	if app.Registry == nil {
		t.Fatal("Registry is nil")
	}
	if buf == nil {
		t.Fatal("buffer is nil")
	}
}

func TestParseFlags_KeyValue(t *testing.T) {
	args := []string{"myname", "--shell=bash", "--no-sync", "--unknown"}
	flags, pos := parseFlags(args, "shell", "no-sync")

	if flags["shell"] != "bash" {
		t.Errorf("shell=%q, want bash", flags["shell"])
	}
	if flags["no-sync"] != "true" {
		t.Errorf("no-sync=%q, want true", flags["no-sync"])
	}
	if len(pos) != 2 || pos[0] != "myname" || pos[1] != "--unknown" {
		t.Errorf("positional=%v, want [myname --unknown]", pos)
	}
}

func TestParseFlags_SpaceSeparated(t *testing.T) {
	args := []string{"--on", "personal", "some-term"}
	flags, pos := parseFlags(args, "on")

	if flags["on"] != "personal" {
		t.Errorf("on=%q, want personal", flags["on"])
	}
	if len(pos) != 1 || pos[0] != "some-term" {
		t.Errorf("positional=%v, want [some-term]", pos)
	}
}

func TestParseFlags_Empty(t *testing.T) {
	flags, pos := parseFlags(nil, "shell")
	if len(flags) != 0 {
		t.Errorf("flags should be empty, got %v", flags)
	}
	if len(pos) != 0 {
		t.Errorf("positional should be empty, got %v", pos)
	}
}
