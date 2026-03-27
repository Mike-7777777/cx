package statusline

import (
	"encoding/json"
	"fmt"
	"io"
)

// Model identifies the Claude model in use.
// Supports both object form {"id":"...","display_name":"..."} (CC <=2.1.81)
// and plain string form "claude-opus-4-6" (CC >=2.1.83).
type Model struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// UnmarshalJSON handles both string and object forms of the model field.
func (m *Model) UnmarshalJSON(data []byte) error {
	// Try string first (CC v2.1.83+).
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		m.ID = s
		m.DisplayName = s
		return nil
	}
	// Fall back to object form.
	type modelAlias Model
	var obj modelAlias
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	*m = Model(obj)
	return nil
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

// OutputStyle indicates the current CC output mode (e.g., "default", "fast").
type OutputStyle struct {
	Name string `json:"name"`
}

// Input is the JSON structure piped from Claude Code on stdin.
type Input struct {
	Model         Model            `json:"model"`
	ContextWindow ContextWindow    `json:"context_window"`
	RateLimits    *InputRateLimits `json:"rate_limits,omitempty"`
	Cost          *Cost            `json:"cost,omitempty"`
	OutputStyle   *OutputStyle     `json:"output_style,omitempty"`
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

// ParseInputBytes decodes the JSON statusline payload from raw bytes.
func ParseInputBytes(data []byte) (*Input, error) {
	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("decoding statusline input: %w", err)
	}
	return &input, nil
}
