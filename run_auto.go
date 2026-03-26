package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/Mike-7777777/cx/internal/cache"
	"github.com/Mike-7777777/cx/internal/usage"
)

// autoCmd implements Runner for the "auto" subcommand.
type autoCmd struct{}

const (
	defaultAutoInterval  = 30 * time.Second
	defaultAutoThreshold = 80.0
	predictWindow        = 15 * time.Minute
)

// Recommendation is the best-account suggestion written to disk for other tools.
type Recommendation struct {
	Account   string    `json:"account"`
	Reason    string    `json:"reason"`
	UpdatedAt time.Time `json:"updated_at"`
}

// shouldSwitch returns true if the account should be switched away from.
// Triggers when:
//   - Usage exceeds threshold, OR
//   - Account is exhausted, OR
//   - Predicted to exhaust within predictWindow (15 minutes)
func shouldSwitch(pct float64, estimate usage.ExhaustionEstimate, threshold float64) bool {
	if estimate.Exhausted {
		return true
	}
	if pct >= threshold {
		return true
	}
	if estimate.TimeToExhaust > 0 && estimate.TimeToExhaust <= predictWindow {
		return true
	}
	return false
}

// recommendationPath returns the path to the recommendation file.
func recommendationPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cx-auto-recommendation.json")
}

// writeRecommendation marshals rec to JSON and writes it to disk.
func writeRecommendation(rec Recommendation) error {
	p := recommendationPath()
	if p == "" {
		return fmt.Errorf("could not determine home directory")
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

// ReadRecommendation reads the recommendation file from disk.
// Returns nil if the file is missing or stale (older than 5 minutes).
func ReadRecommendation() *Recommendation {
	p := recommendationPath()
	if p == "" {
		return nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var rec Recommendation
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil
	}
	if time.Since(rec.UpdatedAt) > 5*time.Minute {
		return nil
	}
	return &rec
}

// checkAndRecommend reads all accounts' rate caches, scores them, and returns
// the best account recommendation. The bool indicates whether any account
// needs urgent switching (above threshold, exhausted, or predicted to exhaust soon).
func checkAndRecommend(app *App, threshold float64) (Recommendation, bool) {
	reg := app.Registry
	now := time.Now()

	type scoredAccount struct {
		name     string
		pct      float64
		estimate usage.ExhaustionEstimate
		score    float64
		reason   string
	}

	var candidates []scoredAccount

	for name := range reg.Accounts {
		dir, err := reg.ResolveConfigDir(name)
		if err != nil {
			continue
		}

		var pct float64
		var ttr time.Duration

		rc, err := cache.ReadRateCache(filepath.Join(dir, "rate-cache.json"))
		if err == nil && rc != nil && rc.RateLimits != nil && rc.RateLimits.FiveHour != nil {
			if rc.RateLimits.FiveHour.IsReset() {
				pct = 0
				ttr = 0
			} else {
				pct = rc.RateLimits.FiveHour.UsedPercentage
				ttr = rc.RateLimits.FiveHour.TimeToReset()
			}
		}

		// Calculate 7d headroom for smartScore tiebreaking.
		var headroom float64 = 1.0
		if rc != nil && rc.RateLimits != nil && rc.RateLimits.SevenDay != nil && !rc.RateLimits.SevenDay.IsReset() {
			daysLeft := time.Until(time.Unix(rc.RateLimits.SevenDay.ResetsAt, 0)).Hours() / 24
			headroom = sevenDayHeadroom(rc.RateLimits.SevenDay.UsedPercentage, daysLeft)
		}

		est := usage.EstimateExhaustion(pct, ttr, now)
		score := smartScore(pct, ttr, headroom)

		var reason string
		switch {
		case est.Exhausted:
			reason = "exhausted"
		case pct >= threshold:
			reason = fmt.Sprintf("%.0f%% >= %.0f%% threshold", pct, threshold)
		case est.TimeToExhaust > 0 && est.TimeToExhaust <= predictWindow:
			reason = fmt.Sprintf("exhausts in %s", est.TimeToExhaust.Round(time.Second))
		default:
			reason = fmt.Sprintf("%.0f%% usage", pct)
		}

		candidates = append(candidates, scoredAccount{
			name:     name,
			pct:      pct,
			estimate: est,
			score:    score,
			reason:   reason,
		})
	}

	if len(candidates) == 0 {
		return Recommendation{
			Account:   "",
			Reason:    "no accounts configured",
			UpdatedAt: now,
		}, false
	}

	// Check if any account needs urgent switching.
	urgent := false
	for _, c := range candidates {
		if shouldSwitch(c.pct, c.estimate, threshold) {
			urgent = true
			break
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score < candidates[j].score
		}
		return candidates[i].name < candidates[j].name
	})

	best := candidates[0]
	return Recommendation{
		Account:   best.name,
		Reason:    best.reason,
		UpdatedAt: now,
	}, urgent
}

// Run implements the auto subcommand: daemon that polls rate caches and
// recommends the best account.
func (c *autoCmd) Run(ctx context.Context, app *App, args []string) error {
	flags, _ := parseFlags(args, "interval", "threshold", "once", "help", "h")

	if _, ok := flags["help"]; ok {
		fmt.Fprint(app.Stdout, autoHelpText)
		return nil
	}
	if _, ok := flags["h"]; ok {
		fmt.Fprint(app.Stdout, autoHelpText)
		return nil
	}

	interval := defaultAutoInterval
	if s, ok := flags["interval"]; ok {
		d, err := time.ParseDuration(s)
		if err != nil {
			return fmt.Errorf("invalid --interval %q: %w", s, err)
		}
		interval = d
	}

	threshold := defaultAutoThreshold
	if s, ok := flags["threshold"]; ok {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("invalid --threshold %q: %w", s, err)
		}
		threshold = v
	}

	once := flags["once"] == "true"

	fmt.Fprintf(app.Stderr, "[cx auto] Monitoring %d account(s) every %s (threshold: %.0f%%)\n",
		len(app.Registry.Accounts), interval, threshold)

	doCheck := func() {
		rec, urgent := checkAndRecommend(app, threshold)
		if err := writeRecommendation(rec); err != nil {
			fmt.Fprintf(app.Stderr, "[cx auto] write recommendation: %v\n", err)
			return
		}
		if urgent {
			fmt.Fprintf(app.Stderr, "[cx auto] SWITCH → %s (%s)\n", rec.Account, rec.Reason)
		} else {
			fmt.Fprintf(app.Stderr, "[cx auto] Best: %s (%s)\n", rec.Account, rec.Reason)
		}
	}

	doCheck()

	if once {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
			doCheck()
		}
	}
}

const autoHelpText = `cx auto — automatic account switching daemon

Usage:
  cx auto [options]

Options:
  --interval <dur>     Polling interval (default: 30s)
  --threshold <pct>    Usage threshold for switching (default: 80)
  --once               Run one check and exit (for cron/testing)
  --help, -h           Show this help

Behavior:
  Polls all accounts' rate-cache files every interval.
  Writes best-account recommendation to ~/.cx-auto-recommendation.json.
  Triggers when usage exceeds threshold OR predicted exhaustion within 15m.

Example:
  cx auto                      # daemon with defaults
  cx auto --interval 10s       # faster polling
  cx auto --threshold 70       # switch earlier
  cx auto --once               # one-shot check
`
