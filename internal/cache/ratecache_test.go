package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRateCache_WriteAndRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	now := time.Now().Unix()
	rc := &RateCache{
		Account:   "user@example.com",
		UpdatedAt: now,
		RateLimits: &RateLimits{
			FiveHour: &Window{UsedPercentage: 42.5, ResetsAt: now + 3600},
			SevenDay: &Window{UsedPercentage: 10.0, ResetsAt: now + 86400},
		},
	}

	if err := WriteRateCache(path, rc); err != nil {
		t.Fatalf("WriteRateCache: %v", err)
	}

	got, err := ReadRateCache(path)
	if err != nil {
		t.Fatalf("ReadRateCache: %v", err)
	}
	if got == nil {
		t.Fatal("ReadRateCache returned nil unexpectedly")
	}

	if got.Account != rc.Account {
		t.Errorf("Account: got %q, want %q", got.Account, rc.Account)
	}
	if got.UpdatedAt != rc.UpdatedAt {
		t.Errorf("UpdatedAt: got %d, want %d", got.UpdatedAt, rc.UpdatedAt)
	}
	if got.RateLimits == nil || got.RateLimits.FiveHour == nil {
		t.Fatal("RateLimits.FiveHour is nil after round-trip")
	}
	if got.RateLimits.FiveHour.UsedPercentage != 42.5 {
		t.Errorf("FiveHour.UsedPercentage: got %v, want 42.5", got.RateLimits.FiveHour.UsedPercentage)
	}
}

func TestRateCache_ReadMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	rc, err := ReadRateCache(path)
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if rc != nil {
		t.Fatalf("expected nil cache for missing file, got: %+v", rc)
	}
}

func TestRateCache_ReadCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.json")

	if err := os.WriteFile(path, []byte("this is not valid json }{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	rc, err := ReadRateCache(path)
	if err != nil {
		t.Fatalf("expected nil error for corrupt file, got: %v", err)
	}
	if rc != nil {
		t.Fatalf("expected nil cache for corrupt file, got: %+v", rc)
	}
}

func TestRateCache_StalenessDetection(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour).Unix()
	future := time.Now().Add(1 * time.Hour).Unix()

	wPast := &Window{ResetsAt: past}
	if !wPast.IsReset() {
		t.Error("window with past ResetsAt should be reset")
	}
	if d := wPast.TimeToReset(); d != 0 {
		t.Errorf("TimeToReset for past window should be 0, got %v", d)
	}

	wFuture := &Window{ResetsAt: future}
	if wFuture.IsReset() {
		t.Error("window with future ResetsAt should not be reset")
	}
	if d := wFuture.TimeToReset(); d <= 0 {
		t.Errorf("TimeToReset for future window should be > 0, got %v", d)
	}
}
