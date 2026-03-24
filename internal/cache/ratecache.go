package cache

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/natefinch/atomic"
)

// Window represents a single rate-limit window with usage and reset metadata.
type Window struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"` // Unix timestamp (seconds)
}

// IsReset returns true if the window's reset time has already passed.
func (w *Window) IsReset() bool {
	return time.Now().Unix() > w.ResetsAt
}

// TimeToReset returns how long until the window resets.
// Returns 0 if already reset.
func (w *Window) TimeToReset() time.Duration {
	delta := time.Until(time.Unix(w.ResetsAt, 0))
	if delta < 0 {
		return 0
	}
	return delta
}

// RateLimits groups the two Claude Code rate-limit windows.
type RateLimits struct {
	FiveHour *Window `json:"five_hour,omitempty"`
	SevenDay *Window `json:"seven_day,omitempty"`
}

// RateCache is the on-disk cache structure written per account.
type RateCache struct {
	Account    string      `json:"account"`
	UpdatedAt  int64       `json:"updated_at"` // Unix timestamp (seconds)
	RateLimits *RateLimits `json:"rate_limits,omitempty"`
}

// Age returns how long ago the cache was written.
func (rc *RateCache) Age() time.Duration {
	return time.Since(time.Unix(rc.UpdatedAt, 0))
}

// WriteRateCache serialises rc to path atomically (write-to-temp + rename).
func WriteRateCache(path string, rc *RateCache) error {
	data, err := json.Marshal(rc)
	if err != nil {
		return err
	}
	return atomic.WriteFile(path, bytes.NewReader(data))
}

// ReadRateCache reads and deserialises the cache at path.
// Returns (nil, nil) when the file is missing or the JSON is corrupt.
func ReadRateCache(path string) (*RateCache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var rc RateCache
	if err := json.Unmarshal(data, &rc); err != nil {
		return nil, nil // treat corrupt cache as absent
	}
	return &rc, nil
}
