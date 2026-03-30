package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/Mike-7777777/cx/internal/cache"
	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/usage"
)

// fiveHourStats reads the 5-hour rate limit window from an account's rate-cache file.
// Returns the usage percentage, time-to-reset, the full RateCache, and whether data was found.
func fiveHourStats(configDir string) (pct float64, ttr time.Duration, rc *cache.RateCache, ok bool) {
	rc, err := cache.ReadRateCache(filepath.Join(configDir, "rate-cache.json"))
	if err != nil || rc == nil || rc.RateLimits == nil || rc.RateLimits.FiveHour == nil {
		return 0, 0, rc, false
	}
	if rc.RateLimits.FiveHour.IsReset() {
		return 0, 0, rc, true
	}
	return rc.RateLimits.FiveHour.UsedPercentage, rc.RateLimits.FiveHour.TimeToReset(), rc, true
}

// parseSinceDate parses a YYYY-MM-DD date string for --since filtering.
func parseSinceDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --since date %q: %v", s, err)
	}
	return t, nil
}

// filterEntriesSince returns entries at or after the given time.
// Returns the original slice unchanged if since is zero.
func filterEntriesSince(entries []usage.Entry, since time.Time) []usage.Entry {
	if since.IsZero() {
		return entries
	}
	result := make([]usage.Entry, 0, len(entries))
	for _, e := range entries {
		if !e.Timestamp.Before(since) {
			result = append(result, e)
		}
	}
	return result
}

// sortedAccountNames returns account names sorted alphabetically with the main
// account first. Used across multiple commands for deterministic iteration.
func sortedAccountNames(reg *config.Registry) []string {
	names := make([]string, 0, len(reg.Accounts))
	for name := range reg.Accounts {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if names[i] == reg.Main {
			return true
		}
		if names[j] == reg.Main {
			return false
		}
		return names[i] < names[j]
	})
	return names
}

// sevenDayHeadroomFromCache extracts the 7-day headroom multiplier from a rate cache.
// Returns 1.0 if no 7-day data is available.
func sevenDayHeadroomFromCache(rc *cache.RateCache) float64 {
	if rc == nil || rc.RateLimits == nil || rc.RateLimits.SevenDay == nil || rc.RateLimits.SevenDay.IsReset() {
		return 1.0
	}
	daysLeft := time.Until(time.Unix(rc.RateLimits.SevenDay.ResetsAt, 0)).Hours() / 24
	return sevenDayHeadroom(rc.RateLimits.SevenDay.UsedPercentage, daysLeft)
}
