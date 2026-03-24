package statusline

import (
	"os"
	"testing"
)

func TestParseInput_Full(t *testing.T) {
	f, err := os.Open("../../testdata/statusline_full.json")
	if err != nil {
		t.Fatalf("open testdata: %v", err)
	}
	defer f.Close()

	input, err := ParseInput(f)
	if err != nil {
		t.Fatalf("ParseInput: %v", err)
	}

	if input.Model.DisplayName != "Opus 4.6" {
		t.Errorf("Model.DisplayName = %q, want %q", input.Model.DisplayName, "Opus 4.6")
	}
	if input.Model.ID != "claude-opus-4-6" {
		t.Errorf("Model.ID = %q, want %q", input.Model.ID, "claude-opus-4-6")
	}

	if input.ContextWindow.UsedPercentage == nil {
		t.Fatal("ContextWindow.UsedPercentage is nil, want 32.5")
	}
	if *input.ContextWindow.UsedPercentage != 32.5 {
		t.Errorf("ContextWindow.UsedPercentage = %v, want 32.5", *input.ContextWindow.UsedPercentage)
	}
	if input.ContextWindow.ContextWindowSize == nil {
		t.Fatal("ContextWindow.ContextWindowSize is nil, want 200000")
	}
	if *input.ContextWindow.ContextWindowSize != 200000 {
		t.Errorf("ContextWindow.ContextWindowSize = %v, want 200000", *input.ContextWindow.ContextWindowSize)
	}

	if input.RateLimits == nil {
		t.Fatal("RateLimits is nil")
	}
	if input.RateLimits.FiveHour == nil {
		t.Fatal("RateLimits.FiveHour is nil")
	}
	if input.RateLimits.FiveHour.UsedPercentage != 38.2 {
		t.Errorf("FiveHour.UsedPercentage = %v, want 38.2", input.RateLimits.FiveHour.UsedPercentage)
	}
	if input.RateLimits.FiveHour.ResetsAt != 9999999999 {
		t.Errorf("FiveHour.ResetsAt = %v, want 9999999999", input.RateLimits.FiveHour.ResetsAt)
	}
	if input.RateLimits.SevenDay == nil {
		t.Fatal("RateLimits.SevenDay is nil")
	}
	if input.RateLimits.SevenDay.UsedPercentage != 62.0 {
		t.Errorf("SevenDay.UsedPercentage = %v, want 62.0", input.RateLimits.SevenDay.UsedPercentage)
	}

	if input.Cost == nil {
		t.Fatal("Cost is nil")
	}
	if input.Cost.TotalCostUSD == nil || *input.Cost.TotalCostUSD != 0.45 {
		t.Errorf("Cost.TotalCostUSD = %v, want 0.45", input.Cost)
	}

	if input.Version != "2.1.81" {
		t.Errorf("Version = %q, want %q", input.Version, "2.1.81")
	}
}

func TestParseInput_Minimal(t *testing.T) {
	f, err := os.Open("../../testdata/statusline_minimal.json")
	if err != nil {
		t.Fatalf("open testdata: %v", err)
	}
	defer f.Close()

	input, err := ParseInput(f)
	if err != nil {
		t.Fatalf("ParseInput: %v", err)
	}

	if input.Model.DisplayName != "Opus 4.6" {
		t.Errorf("Model.DisplayName = %q, want %q", input.Model.DisplayName, "Opus 4.6")
	}
	if input.RateLimits != nil {
		t.Errorf("RateLimits = %v, want nil", input.RateLimits)
	}
	if input.Cost != nil {
		t.Errorf("Cost = %v, want nil", input.Cost)
	}
	if input.ContextWindow.UsedPercentage == nil {
		t.Fatal("ContextWindow.UsedPercentage is nil, want 15.0")
	}
	if *input.ContextWindow.UsedPercentage != 15.0 {
		t.Errorf("ContextWindow.UsedPercentage = %v, want 15.0", *input.ContextWindow.UsedPercentage)
	}
}

func TestParseInput_Null(t *testing.T) {
	f, err := os.Open("../../testdata/statusline_null.json")
	if err != nil {
		t.Fatalf("open testdata: %v", err)
	}
	defer f.Close()

	input, err := ParseInput(f)
	if err != nil {
		t.Fatalf("ParseInput: %v", err)
	}

	if input.Model.DisplayName != "Opus 4.6" {
		t.Errorf("Model.DisplayName = %q, want %q", input.Model.DisplayName, "Opus 4.6")
	}
	if input.ContextWindow.UsedPercentage != nil {
		t.Errorf("ContextWindow.UsedPercentage = %v, want nil", input.ContextWindow.UsedPercentage)
	}
	if input.ContextWindow.ContextWindowSize != nil {
		t.Errorf("ContextWindow.ContextWindowSize = %v, want nil", input.ContextWindow.ContextWindowSize)
	}
	if input.RateLimits != nil {
		t.Errorf("RateLimits = %v, want nil", input.RateLimits)
	}
}
