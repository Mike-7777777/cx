package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/Mike-7777777/cx/internal/config"
)

func TestInit_InvalidName(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{Main: "main", Accounts: map[string]config.Account{}},
		Stdout:   &buf, Stderr: &buf, UseColor: false,
	}
	cmd := &initCmd{}
	err := cmd.Run(context.Background(), app, []string{"../evil"})
	if err == nil {
		t.Fatal("expected error for invalid name")
	}
}

func TestInit_NoArgs(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{Main: "main", Accounts: map[string]config.Account{}},
		Stdout:   &buf, Stderr: &buf, UseColor: false,
	}
	cmd := &initCmd{}
	err := cmd.Run(context.Background(), app, nil)
	if err == nil {
		t.Fatal("expected error when no name provided")
	}
}
