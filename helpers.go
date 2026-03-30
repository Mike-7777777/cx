package main

import (
	"path/filepath"
	"time"

	"github.com/Mike-7777777/cx/internal/cache"
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

// sevenDayHeadroomFromCache extracts the 7-day headroom multiplier from a rate cache.
// Returns 1.0 if no 7-day data is available.
func sevenDayHeadroomFromCache(rc *cache.RateCache) float64 {
	if rc == nil || rc.RateLimits == nil || rc.RateLimits.SevenDay == nil || rc.RateLimits.SevenDay.IsReset() {
		return 1.0
	}
	daysLeft := time.Until(time.Unix(rc.RateLimits.SevenDay.ResetsAt, 0)).Hours() / 24
	return sevenDayHeadroom(rc.RateLimits.SevenDay.UsedPercentage, daysLeft)
}
