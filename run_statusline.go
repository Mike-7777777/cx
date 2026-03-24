package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/MasaYan24/cc-monitor/internal/cache"
	"github.com/MasaYan24/cc-monitor/internal/config"
	"github.com/MasaYan24/cc-monitor/internal/platform"
	"github.com/MasaYan24/cc-monitor/internal/statusline"
)

func runStatusline() {
	// On ANY error: print [?] and exit 0 (never blank the status bar).
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("[?]")
		}
	}()

	if err := doStatusline(); err != nil {
		logError(err)
		fmt.Println("[?]")
	}
}

func doStatusline() error {
	input, err := statusline.ParseInput(os.Stdin)
	if err != nil {
		return fmt.Errorf("parse stdin: %w", err)
	}

	cfgDir, err := config.DetectConfigDir()
	if err != nil {
		// No config dir is non-fatal; render without cache ops.
		return renderAndPrint(input, nil)
	}

	// Write current account's rate-cache.json if rate_limits present.
	if input.RateLimits != nil {
		rc := buildRateCache(input)
		cachePath := filepath.Join(cfgDir, "rate-cache.json")
		if wErr := cache.WriteRateCache(cachePath, rc); wErr != nil {
			logError(fmt.Errorf("write rate cache: %w", wErr))
			// Non-fatal: continue rendering.
		}
	}

	// Read other accounts' rate-cache.json.
	other := loadOtherAccount(cfgDir)

	return renderAndPrint(input, other)
}

func buildRateCache(input *statusline.Input) *cache.RateCache {
	rc := &cache.RateCache{
		UpdatedAt: time.Now().Unix(),
	}

	rl := &cache.RateLimits{}
	if input.RateLimits.FiveHour != nil {
		rl.FiveHour = &cache.Window{
			UsedPercentage: input.RateLimits.FiveHour.UsedPercentage,
			ResetsAt:       input.RateLimits.FiveHour.ResetsAt,
		}
	}
	if input.RateLimits.SevenDay != nil {
		rl.SevenDay = &cache.Window{
			UsedPercentage: input.RateLimits.SevenDay.UsedPercentage,
			ResetsAt:       input.RateLimits.SevenDay.ResetsAt,
		}
	}
	rc.RateLimits = rl
	return rc
}

func loadOtherAccount(currentCfgDir string) *statusline.OtherAccount {
	regPath, err := config.RegistryPath()
	if err != nil {
		return nil
	}

	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		return nil
	}

	for name, acc := range reg.Accounts {
		accDir := acc.ConfigDir
		if accDir == "" {
			continue
		}
		// Skip current account.
		if accDir == currentCfgDir {
			continue
		}

		cachePath := filepath.Join(accDir, "rate-cache.json")
		rc, err := cache.ReadRateCache(cachePath)
		if err != nil || rc == nil {
			continue
		}

		other := &statusline.OtherAccount{Name: name}

		if rc.RateLimits != nil {
			if rc.RateLimits.FiveHour != nil {
				other.FiveHour = rc.RateLimits.FiveHour.UsedPercentage
				if rc.RateLimits.FiveHour.IsReset() {
					other.Stale = "reset"
				}
			}
			if rc.RateLimits.SevenDay != nil {
				other.SevenDay = rc.RateLimits.SevenDay.UsedPercentage
			}
		}

		// Staleness detection: if not reset, check cache age.
		if other.Stale == "" {
			age := rc.Age()
			if age > 30*time.Minute {
				other.Stale = formatStaleAge(age)
			}
		}

		return other // Only show first other account found.
	}

	return nil
}

func formatStaleAge(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func renderAndPrint(input *statusline.Input, other *statusline.OtherAccount) error {
	lines := statusline.Render(input, other, platform.ANSIEnabled())
	for _, line := range lines {
		fmt.Println(line)
	}
	return nil
}

func logError(err error) {
	cfgDir, dirErr := config.DetectConfigDir()
	if dirErr != nil {
		return // Cannot log if we don't know where to write.
	}

	logPath := filepath.Join(cfgDir, "cc-monitor.log")
	f, fErr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if fErr != nil {
		return
	}
	defer f.Close()

	logger := log.New(f, "", log.LstdFlags)
	logger.Printf("ERROR: %v", err)
}
