package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mike-7777777/cx/internal/cache"
	"github.com/Mike-7777777/cx/internal/config"
)

func TestPredictCmd_ShowsForecast(t *testing.T) {
	dir := t.TempDir()

	// Write a rate-cache with 40% usage and 2 hours remaining.
	resetsAt := time.Now().Add(2 * time.Hour).Unix()
	rc := &cache.RateCache{
		Account:   "testaccount",
		UpdatedAt: time.Now().Unix(),
		RateLimits: &cache.RateLimits{
			FiveHour: &cache.Window{
				UsedPercentage: 40.0,
				ResetsAt:       resetsAt,
			},
		},
	}
	if err := cache.WriteRateCache(filepath.Join(dir, "rate-cache.json"), rc); err != nil {
		t.Fatalf("WriteRateCache: %v", err)
	}

	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{
			Main: "testaccount",
			Accounts: map[string]config.Account{
				"testaccount": {ConfigDir: dir},
			},
		},
		Stdout:   &buf,
		Stderr:   &buf,
		UseColor: false,
	}

	cmd := &predictCmd{}
	if err := cmd.Run(context.Background(), app, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "testaccount") {
		t.Errorf("output missing account name: %q", out)
	}
	if !strings.Contains(out, "40") {
		t.Errorf("output missing 40%% usage: %q", out)
	}
}

func TestPredictCmd_JSONOutput(t *testing.T) {
	dir := t.TempDir()

	resetsAt := time.Now().Add(3 * time.Hour).Unix()
	rc := &cache.RateCache{
		Account:   "jsonaccount",
		UpdatedAt: time.Now().Unix(),
		RateLimits: &cache.RateLimits{
			FiveHour: &cache.Window{
				UsedPercentage: 60.0,
				ResetsAt:       resetsAt,
			},
		},
	}
	if err := cache.WriteRateCache(filepath.Join(dir, "rate-cache.json"), rc); err != nil {
		t.Fatalf("WriteRateCache: %v", err)
	}

	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{
			Main: "jsonaccount",
			Accounts: map[string]config.Account{
				"jsonaccount": {ConfigDir: dir},
			},
		},
		Stdout:   &buf,
		Stderr:   &buf,
		UseColor: false,
	}

	cmd := &predictCmd{}
	if err := cmd.Run(context.Background(), app, []string{"--json"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 JSON entry, got %d", len(result))
	}
	if result[0]["account"] != "jsonaccount" {
		t.Errorf("JSON account field: got %v, want jsonaccount", result[0]["account"])
	}
}

func TestPredictCmd_NoAccounts(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{
			Accounts: map[string]config.Account{},
		},
		Stdout:   &buf,
		Stderr:   &buf,
		UseColor: false,
	}

	cmd := &predictCmd{}
	if err := cmd.Run(context.Background(), app, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(buf.String(), "No accounts") {
		t.Errorf("expected 'No accounts' message, got: %q", buf.String())
	}
}

func TestPredictCmd_Help(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Registry: &config.Registry{Accounts: map[string]config.Account{}},
		Stdout:   &buf,
		Stderr:   &buf,
		UseColor: false,
	}

	cmd := &predictCmd{}
	if err := cmd.Run(context.Background(), app, []string{"--help"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(buf.String(), "forecast") {
		t.Errorf("help text missing 'forecast': %q", buf.String())
	}
}
