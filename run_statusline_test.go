package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Mike-7777777/cx/internal/cache"
	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/statusline"
)

// --- hasFlag ---

func TestHasFlag_Present(t *testing.T) {
	if !hasFlag([]string{"--compact", "--json"}, "--compact") {
		t.Error("expected true for present flag")
	}
}

func TestHasFlag_Absent(t *testing.T) {
	if hasFlag([]string{"--compact"}, "--json") {
		t.Error("expected false for absent flag")
	}
}

func TestHasFlag_EmptyArgs(t *testing.T) {
	if hasFlag(nil, "--compact") {
		t.Error("expected false for nil args")
	}
}

// --- normalizePath ---

func TestNormalizePath_ForwardSlashes(t *testing.T) {
	p := normalizePath(`/home/user/.config`)
	if strings.Contains(p, `\`) {
		t.Errorf("expected no backslashes, got %q", p)
	}
}

func TestNormalizePath_BackslashConverted(t *testing.T) {
	p := normalizePath(`C:\Users\test\.config`)
	if strings.Contains(p, `\`) {
		t.Errorf("expected forward slashes, got %q", p)
	}
}

func TestNormalizePath_WindowsLowercase(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	a := normalizePath(`C:\Users\Test`)
	b := normalizePath(`c:\users\test`)
	if a != b {
		t.Errorf("Windows paths should match case-insensitively: %q vs %q", a, b)
	}
}

func TestNormalizePath_CleansDots(t *testing.T) {
	p := normalizePath("/home/user/../user/.config")
	if strings.Contains(p, "..") {
		t.Errorf("expected cleaned path, got %q", p)
	}
}

// --- buildRateCache ---

func TestBuildRateCache_BothWindows(t *testing.T) {
	input := &statusline.Input{
		RateLimits: &statusline.InputRateLimits{
			FiveHour: &statusline.RateWindow{UsedPercentage: 42.0, ResetsAt: 1234567890},
			SevenDay: &statusline.RateWindow{UsedPercentage: 15.0, ResetsAt: 9876543210},
		},
	}
	rc := buildRateCache(input)
	if rc.RateLimits == nil {
		t.Fatal("RateLimits should not be nil")
	}
	if rc.RateLimits.FiveHour == nil {
		t.Fatal("FiveHour should not be nil")
	}
	if rc.RateLimits.FiveHour.UsedPercentage != 42.0 {
		t.Errorf("FiveHour.UsedPercentage = %f, want 42.0", rc.RateLimits.FiveHour.UsedPercentage)
	}
	if rc.RateLimits.SevenDay == nil {
		t.Fatal("SevenDay should not be nil")
	}
	if rc.RateLimits.SevenDay.UsedPercentage != 15.0 {
		t.Errorf("SevenDay.UsedPercentage = %f, want 15.0", rc.RateLimits.SevenDay.UsedPercentage)
	}
	if rc.UpdatedAt == 0 {
		t.Error("UpdatedAt should be set")
	}
}

func TestBuildRateCache_FiveHourOnly(t *testing.T) {
	input := &statusline.Input{
		RateLimits: &statusline.InputRateLimits{
			FiveHour: &statusline.RateWindow{UsedPercentage: 10.0, ResetsAt: 100},
		},
	}
	rc := buildRateCache(input)
	if rc.RateLimits.FiveHour == nil {
		t.Error("FiveHour should not be nil")
	}
	if rc.RateLimits.SevenDay != nil {
		t.Error("SevenDay should be nil when not provided")
	}
}

func TestBuildRateCache_NoWindows(t *testing.T) {
	input := &statusline.Input{
		RateLimits: &statusline.InputRateLimits{},
	}
	rc := buildRateCache(input)
	if rc.RateLimits.FiveHour != nil {
		t.Error("FiveHour should be nil")
	}
	if rc.RateLimits.SevenDay != nil {
		t.Error("SevenDay should be nil")
	}
}

// --- readEffortFromFile ---

func TestReadEffortFromFile_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	data := `{"effortLevel":"low"}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	got := readEffortFromFile(path)
	if got != "low" {
		t.Errorf("got %q, want %q", got, "low")
	}
}

func TestReadEffortFromFile_NoEffortField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	data := `{"someOtherField":"value"}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	got := readEffortFromFile(path)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestReadEffortFromFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := readEffortFromFile(path)
	if got != "" {
		t.Errorf("got %q, want empty for invalid JSON", got)
	}
}

func TestReadEffortFromFile_MissingFile(t *testing.T) {
	got := readEffortFromFile("/nonexistent/settings.json")
	if got != "" {
		t.Errorf("got %q, want empty for missing file", got)
	}
}

// --- readEffortLevel ---

func TestReadEffortLevel_EmptyCfgDir(t *testing.T) {
	got := readEffortLevel("")
	if got != "" {
		t.Errorf("got %q, want empty for empty cfgDir", got)
	}
}

func TestReadEffortLevel_AccountLevel(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	data := `{"effortLevel":"high"}`
	if err := os.WriteFile(settingsPath, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	got := readEffortLevel(dir)
	if got != "high" {
		t.Errorf("got %q, want %q", got, "high")
	}
}

// --- rotateLogIfNeeded ---

func TestRotateLogIfNeeded_SmallFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "cx.log")
	if err := os.WriteFile(logPath, []byte("small log"), 0o644); err != nil {
		t.Fatal(err)
	}
	rotateLogIfNeeded(logPath)

	// File should still exist (not rotated).
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("log file should still exist: %v", err)
	}
	// No .old file should exist.
	if _, err := os.Stat(logPath + ".old"); !os.IsNotExist(err) {
		t.Error(".old file should not exist for small log")
	}
}

func TestRotateLogIfNeeded_LargeFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "cx.log")
	// Create a file larger than logMaxSize (1 MB).
	data := make([]byte, logMaxSize+100)
	if err := os.WriteFile(logPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	rotateLogIfNeeded(logPath)

	// Original file should be gone (renamed to .old).
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Error("original log should be renamed")
	}
	// .old file should exist.
	info, err := os.Stat(logPath + ".old")
	if err != nil {
		t.Fatalf(".old file should exist: %v", err)
	}
	if info.Size() != int64(logMaxSize+100) {
		t.Errorf(".old size = %d, want %d", info.Size(), logMaxSize+100)
	}
}

func TestRotateLogIfNeeded_MissingFile(t *testing.T) {
	// Should not panic for non-existent file.
	rotateLogIfNeeded("/nonexistent/cx.log")
}

// --- openLogFile ---

func TestOpenLogFile_CreatesNew(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "cx.log")
	f, err := openLogFile(logPath)
	if err != nil {
		t.Fatalf("openLogFile: %v", err)
	}
	_ = f.Close()

	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("log file should be created: %v", err)
	}
}

// --- currentAccountName ---

func TestCurrentAccountName_Match(t *testing.T) {
	fakeHome := t.TempDir()
	configDir := filepath.Join(fakeHome, "claude-config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	reg := &config.Registry{
		Version: 1,
		Main:    "primary",
		Accounts: map[string]config.Account{
			"primary":   {ConfigDir: configDir},
			"secondary": {ConfigDir: filepath.Join(fakeHome, "other")},
		},
	}
	regData, _ := json.MarshalIndent(reg, "", "  ")
	if err := os.WriteFile(filepath.Join(fakeHome, ".cx.json"), regData, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", fakeHome)
	t.Setenv("USERPROFILE", fakeHome)

	got := currentAccountName(configDir)
	if got != "primary" {
		t.Errorf("got %q, want %q", got, "primary")
	}
}

func TestCurrentAccountName_NoMatch(t *testing.T) {
	fakeHome := t.TempDir()
	reg := &config.Registry{
		Version: 1,
		Main:    "only",
		Accounts: map[string]config.Account{
			"only": {ConfigDir: filepath.Join(fakeHome, "some-dir")},
		},
	}
	regData, _ := json.MarshalIndent(reg, "", "  ")
	if err := os.WriteFile(filepath.Join(fakeHome, ".cx.json"), regData, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", fakeHome)
	t.Setenv("USERPROFILE", fakeHome)

	got := currentAccountName(filepath.Join(fakeHome, "unrelated-dir"))
	if got != "" {
		t.Errorf("got %q, want empty for non-matching dir", got)
	}
}

// --- loadOtherAccount ---

func TestLoadOtherAccount_ReturnsHighestUsage(t *testing.T) {
	fakeHome := t.TempDir()
	currentDir := filepath.Join(fakeHome, "current")
	otherDir1 := filepath.Join(fakeHome, "other1")
	otherDir2 := filepath.Join(fakeHome, "other2")

	for _, dir := range []string{currentDir, otherDir1, otherDir2} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Write rate caches: other1 at 30%, other2 at 70%.
	writeTestRateCache(t, otherDir1, 30.0)
	writeTestRateCache(t, otherDir2, 70.0)

	reg := &config.Registry{
		Version: 1,
		Main:    "current",
		Accounts: map[string]config.Account{
			"current": {ConfigDir: currentDir},
			"low":     {ConfigDir: otherDir1},
			"high":    {ConfigDir: otherDir2},
		},
	}
	regData, _ := json.MarshalIndent(reg, "", "  ")
	if err := os.WriteFile(filepath.Join(fakeHome, ".cx.json"), regData, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", fakeHome)
	t.Setenv("USERPROFILE", fakeHome)

	other := loadOtherAccount(currentDir)
	if other == nil {
		t.Fatal("expected non-nil other account")
	}
	if other.Name != "high" {
		t.Errorf("got %q, want %q (highest usage)", other.Name, "high")
	}
	if other.FiveHour != 70.0 {
		t.Errorf("FiveHour = %f, want 70.0", other.FiveHour)
	}
}

func TestLoadOtherAccount_NoOtherAccounts(t *testing.T) {
	fakeHome := t.TempDir()
	configDir := filepath.Join(fakeHome, "only")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	reg := &config.Registry{
		Version: 1,
		Main:    "only",
		Accounts: map[string]config.Account{
			"only": {ConfigDir: configDir},
		},
	}
	regData, _ := json.MarshalIndent(reg, "", "  ")
	if err := os.WriteFile(filepath.Join(fakeHome, ".cx.json"), regData, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", fakeHome)
	t.Setenv("USERPROFILE", fakeHome)

	other := loadOtherAccount(configDir)
	if other != nil {
		t.Errorf("expected nil for single account, got %+v", other)
	}
}

// writeTestRateCache creates a rate-cache.json with the given 5h usage.
func writeTestRateCache(t *testing.T, dir string, fiveHourPct float64) {
	t.Helper()
	rc := &cache.RateCache{
		UpdatedAt: 0, // age will be large, but that's OK for these tests
		RateLimits: &cache.RateLimits{
			FiveHour: &cache.Window{
				UsedPercentage: fiveHourPct,
				ResetsAt:       9999999999, // far future
			},
		},
	}
	if err := cache.WriteRateCache(filepath.Join(dir, "rate-cache.json"), rc); err != nil {
		t.Fatalf("WriteRateCache: %v", err)
	}
}
