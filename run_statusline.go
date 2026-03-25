package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/Mike-7777777/cc-monitor/internal/cache"
	"github.com/Mike-7777777/cc-monitor/internal/config"
	"github.com/Mike-7777777/cc-monitor/internal/platform"
	"github.com/Mike-7777777/cc-monitor/internal/statusline"
)

const (
	logMaxSize      = 1048576 // 1 MB
	minVersionWarn  = "2.1.80"
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

	// Warn when rate_limits is absent and CC version is too old.
	if input.RateLimits == nil && input.Version != "" && input.Version < minVersionWarn {
		logWarn(fmt.Sprintf(
			"Claude Code version %s may not provide rate_limits data. Recommended: >= %s",
			input.Version, minVersionWarn,
		))
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

// rotateLogIfNeeded rotates logPath when it exceeds logMaxSize bytes.
// It removes the .old file if present, then renames the current log to .old.
func rotateLogIfNeeded(logPath string) {
	info, err := os.Stat(logPath)
	if err != nil || info.Size() <= logMaxSize {
		return
	}
	oldPath := logPath + ".old"
	_ = os.Remove(oldPath)
	_ = os.Rename(logPath, oldPath)
}

// openLogFile opens (or creates) the log file at logPath, rotating first if needed.
func openLogFile(logPath string) (*os.File, error) {
	rotateLogIfNeeded(logPath)
	return os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
}

func logError(err error) {
	cfgDir, dirErr := config.DetectConfigDir()
	if dirErr != nil {
		return // Cannot log if we don't know where to write.
	}

	logPath := filepath.Join(cfgDir, "cc-monitor.log")
	f, fErr := openLogFile(logPath)
	if fErr != nil {
		return
	}
	defer f.Close()

	logger := log.New(f, "", log.LstdFlags)
	logger.Printf("ERROR: %v", err)
}

func logWarn(msg string) {
	cfgDir, dirErr := config.DetectConfigDir()
	if dirErr != nil {
		return
	}

	logPath := filepath.Join(cfgDir, "cc-monitor.log")
	f, fErr := openLogFile(logPath)
	if fErr != nil {
		return
	}
	defer f.Close()

	logger := log.New(f, "", log.LstdFlags)
	logger.Printf("[WARN] %s", msg)
}
