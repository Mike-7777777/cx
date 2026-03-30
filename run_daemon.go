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
	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/usage"
)

// daemonCmd implements Runner for the "daemon" subcommand.
// Combines auto-switching and config-sync into one process.
type daemonCmd struct{}

const (
	defaultDaemonInterval  = 30 * time.Second
	defaultDaemonThreshold = 80.0
	daemonSyncInterval     = 30 * time.Second
	predictWindow          = 15 * time.Minute
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

// --- Watch helpers (moved from run_watch.go) ---

// watchState tracks the last-seen mtime and size of each synced file.
type watchState map[string]watchFileMeta

type watchFileMeta struct {
	mtime time.Time
	size  int64
}

// snapshotFiles records the mtime and size of every file in syncFileList.
func snapshotFiles(dir string) watchState {
	state := make(watchState, len(syncFileList))
	for _, rel := range syncFileList {
		path := filepath.Join(dir, rel)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		state[rel] = watchFileMeta{
			mtime: info.ModTime(),
			size:  info.Size(),
		}
	}
	return state
}

// detectChanges compares current file metadata against the stored snapshot.
func detectChanges(dir string, prev watchState) []string {
	var changed []string
	for _, rel := range syncFileList {
		path := filepath.Join(dir, rel)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		fileMeta, seen := prev[rel]
		if !seen || info.ModTime().After(fileMeta.mtime) || info.Size() != fileMeta.size {
			changed = append(changed, rel)
		}
	}
	return changed
}

// --- Daemon Run ---

func (c *daemonCmd) Run(ctx context.Context, app *App, args []string) error {
	flags, _ := parseFlags(args, "interval", "threshold", "once", "no-sync", "no-switch", "help", "h")

	if _, ok := flags["help"]; ok {
		fmt.Fprint(app.Stdout, daemonHelpText)
		return nil
	}
	if _, ok := flags["h"]; ok {
		fmt.Fprint(app.Stdout, daemonHelpText)
		return nil
	}

	interval := defaultDaemonInterval
	if s, ok := flags["interval"]; ok {
		d, err := time.ParseDuration(s)
		if err != nil {
			return fmt.Errorf("invalid --interval %q: %w", s, err)
		}
		interval = d
	}

	threshold := defaultDaemonThreshold
	if s, ok := flags["threshold"]; ok {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("invalid --threshold %q: %w", s, err)
		}
		threshold = v
	}

	once := flags["once"] == "true"
	noSync := flags["no-sync"] == "true"
	noSwitch := flags["no-switch"] == "true"

	if noSync && noSwitch {
		return fmt.Errorf("cannot use both --no-sync and --no-switch")
	}

	// Set up sync state if enabled.
	var syncState watchState
	var mainDir string
	var secondaryCount int

	if !noSync {
		regPath, err := config.RegistryPath()
		if err != nil {
			noSync = true
		} else {
			reg, err := config.LoadOrCreateRegistry(regPath)
			if err != nil {
				noSync = true
			} else if reg.Main == "" {
				noSync = true
			} else {
				mainDir, err = reg.ResolveConfigDir(reg.Main)
				if err != nil {
					noSync = true
				} else {
					secondaryCount = len(reg.Accounts) - 1
					if secondaryCount <= 0 {
						noSync = true
					} else {
						syncState = snapshotFiles(mainDir)
					}
				}
			}
		}
	}

	// Print startup banner.
	var features []string
	if !noSwitch {
		features = append(features, fmt.Sprintf("auto-switch (threshold: %.0f%%)", threshold))
	}
	if !noSync {
		features = append(features, fmt.Sprintf("config-sync (%d secondary)", secondaryCount))
	}
	if len(features) == 0 {
		features = append(features, "none")
	}

	featStr := features[0]
	for i := 1; i < len(features); i++ {
		featStr += ", " + features[i]
	}

	fmt.Fprintf(app.Stderr, "[cx daemon] Monitoring %d account(s) every %s [%s]\n",
		len(app.Registry.Accounts), interval, featStr)

	doAutoSwitch := func() {
		rec, urgent := checkAndRecommend(app, threshold)
		if err := writeRecommendation(rec); err != nil {
			fmt.Fprintf(app.Stderr, "[cx daemon] write recommendation: %v\n", err)
			return
		}
		if urgent {
			fmt.Fprintf(app.Stderr, "[cx daemon] SWITCH → %s (%s)\n", rec.Account, rec.Reason)
		} else {
			fmt.Fprintf(app.Stderr, "[cx daemon] Best: %s (%s)\n", rec.Account, rec.Reason)
		}
	}

	doSync := func() {
		changed := detectChanges(mainDir, syncState)
		if len(changed) == 0 {
			return
		}

		regPath, _ := config.RegistryPath()
		reg, err := config.LoadOrCreateRegistry(regPath)
		if err != nil {
			fmt.Fprintf(app.Stderr, "[cx daemon] reloading registry: %v\n", err)
			return
		}

		synced := 0
		for name, acc := range reg.Accounts {
			if name == reg.Main {
				continue
			}
			targetDir := acc.ConfigDir
			if targetDir == "" {
				continue
			}
			if err := syncFiles(mainDir, targetDir, true); err != nil {
				fmt.Fprintf(app.Stderr, "[cx daemon] syncing %q: %v\n", name, err)
				continue
			}
			synced++
		}

		ts := time.Now().Format("2006-01-02 15:04")
		for _, f := range changed {
			fmt.Fprintf(app.Stderr, "[%s] synced %s to %dx\n", ts, f, synced)
		}

		syncState = snapshotFiles(mainDir)
	}

	// Initial check.
	if !noSwitch {
		doAutoSwitch()
	}
	if !noSync {
		doSync()
	}

	if once {
		return nil
	}

	switchTicker := time.NewTicker(interval)
	defer switchTicker.Stop()

	syncTicker := time.NewTicker(daemonSyncInterval)
	defer syncTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-switchTicker.C:
			if !noSwitch {
				doAutoSwitch()
			}
		case <-syncTicker.C:
			if !noSync {
				doSync()
			}
		}
	}
}

const daemonHelpText = `cx daemon — combined auto-switch and config-sync daemon

Usage:
  cx daemon [options]

Options:
  --no-sync            Disable config sync (auto-switch only)
  --no-switch          Disable auto-switch (sync only)
  --interval <dur>     Auto-switch polling interval (default: 30s)
  --threshold <pct>    Usage threshold for switching (default: 80)
  --once               Run one cycle and exit
  --help, -h           Show this help

Behavior:
  Combines the former 'cx auto' and 'cx watch' into one process.
  Auto-switch: polls rate caches, writes recommendation to ~/.cx-auto-recommendation.json.
  Config-sync: watches main account's config files and syncs to secondaries.

Example:
  cx daemon                          # both features, default settings
  cx daemon --no-sync                # auto-switch only
  cx daemon --no-switch              # config sync only
  cx daemon --interval 10s --once    # one-shot with fast interval
`
