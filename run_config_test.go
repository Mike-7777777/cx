package main

import (
	"testing"

	"github.com/Mike-7777777/cx/internal/config"
)

func TestSortedAccountNames_MainFirst(t *testing.T) {
	reg := &config.Registry{
		Main: "beta",
		Accounts: map[string]config.Account{
			"alpha": {},
			"beta":  {},
			"gamma": {},
		},
	}

	names := sortedAccountNames(reg)

	if len(names) != 3 {
		t.Fatalf("got %d names, want 3", len(names))
	}
	if names[0] != "beta" {
		t.Errorf("first name = %q, want %q (main account)", names[0], "beta")
	}
	// Remaining should be alphabetically sorted.
	if names[1] != "alpha" {
		t.Errorf("second name = %q, want %q", names[1], "alpha")
	}
	if names[2] != "gamma" {
		t.Errorf("third name = %q, want %q", names[2], "gamma")
	}
}

func TestSortedAccountNames_SingleAccount(t *testing.T) {
	reg := &config.Registry{
		Main: "only",
		Accounts: map[string]config.Account{
			"only": {},
		},
	}

	names := sortedAccountNames(reg)

	if len(names) != 1 {
		t.Fatalf("got %d names, want 1", len(names))
	}
	if names[0] != "only" {
		t.Errorf("name = %q, want %q", names[0], "only")
	}
}

func TestSortedAccountNames_MainAlreadyFirst(t *testing.T) {
	reg := &config.Registry{
		Main: "aaa",
		Accounts: map[string]config.Account{
			"aaa": {},
			"bbb": {},
			"ccc": {},
		},
	}

	names := sortedAccountNames(reg)

	if names[0] != "aaa" {
		t.Errorf("first name = %q, want %q", names[0], "aaa")
	}
}

func TestResolveDir_Error(t *testing.T) {
	// Create a registry with no accounts; resolving should return "?".
	reg := &config.Registry{
		Accounts: map[string]config.Account{},
	}

	got := resolveDir(reg, "nonexistent")
	if got != "?" {
		t.Errorf("resolveDir for missing account = %q, want %q", got, "?")
	}
}
