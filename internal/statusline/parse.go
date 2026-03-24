package statusline

import (
	"encoding/json"
	"fmt"
	"io"
)

// Model identifies the Claude model in use.
type Model struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// RateWindow holds usage and reset metadata for a single rate-limit window.
type RateWindow struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"`
}

// InputRateLimits groups the two Claude Code rate-limit windows.
type InputRateLimits struct {
	FiveHour *RateWindow `json:"five_hour,omitempty"`
	SevenDay *RateWindow `json:"seven_day,omitempty"`
}

// ContextWindow holds context utilisation. Pointer fields are nil when absent.
type ContextWindow struct {
	UsedPercentage    *float64 `json:"used_percentage"`
	ContextWindowSize *int64   `json:"context_window_size"`
}

// Cost holds session cost data.
type Cost struct {
	TotalCostUSD *float64 `json:"total_cost_usd"`
}

// Input is the JSON structure piped from Claude Code on stdin.
type Input struct {
	Model         Model            `json:"model"`
	ContextWindow ContextWindow    `json:"context_window"`
	RateLimits    *InputRateLimits `json:"rate_limits,omitempty"`
	Cost          *Cost            `json:"cost,omitempty"`
	Version       string           `json:"version"`
}

// ParseInput decodes the JSON statusline payload from r.
func ParseInput(r io.Reader) (*Input, error) {
	var input Input
	if err := json.NewDecoder(r).Decode(&input); err != nil {
		return nil, fmt.Errorf("decoding statusline input: %w", err)
	}
	return &input, nil
}
