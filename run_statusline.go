package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Mike-7777777/cx/internal/cache"
	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/format"
	"github.com/Mike-7777777/cx/internal/platform"
	"github.com/Mike-7777777/cx/internal/statusline"
)

const (
	logMaxSize                     = 1048576 // 1 MB
	minVersionWarn                 = "2.1.80"
	statuslineStaleCacheThreshold  = 30 * time.Minute
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
		// CC may not pipe data to stdin (e.g., Windows Git Bash pipe issue).
		// Fall back to rendering from cached rate data instead of showing [?].
		logWarn(fmt.Sprintf("stdin parse failed (%v), attempting cache fallback", err))
		return doStatuslineFallback()
	}

	compact := hasFlagFrom("--compact", 2)

	cfgDir, err := config.DetectConfigDir()
	if err != nil {
		// No config dir is non-fatal; render without cache ops.
		return renderAndPrint(input, nil, "", compact)
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

	// Resolve current account name from registry.
	accountName := currentAccountName(cfgDir)

	return renderAndPrint(input, other, accountName, compact)
}

// doStatuslineFallback renders a minimal statusline from cached rate data
// when CC fails to pipe JSON to stdin.
func doStatuslineFallback() error {
	compact := hasFlagFrom("--compact", 2)

	cfgDir, err := config.DetectConfigDir()
	if err != nil {
		return fmt.Errorf("no config dir for fallback: %w", err)
	}

	accountName := currentAccountName(cfgDir)

	// Try to build a minimal Input from the rate cache.
	cachePath := filepath.Join(cfgDir, "rate-cache.json")
	rc, rcErr := cache.ReadRateCache(cachePath)

	input := &statusline.Input{
		Model: statusline.Model{ID: "unknown", DisplayName: "?"},
	}

	if rcErr == nil && rc != nil && rc.RateLimits != nil {
		rl := &statusline.InputRateLimits{}
		if rc.RateLimits.FiveHour != nil {
			rl.FiveHour = &statusline.RateWindow{
				UsedPercentage: rc.RateLimits.FiveHour.UsedPercentage,
				ResetsAt:       rc.RateLimits.FiveHour.ResetsAt,
			}
		}
		if rc.RateLimits.SevenDay != nil {
			rl.SevenDay = &statusline.RateWindow{
				UsedPercentage: rc.RateLimits.SevenDay.UsedPercentage,
				ResetsAt:       rc.RateLimits.SevenDay.ResetsAt,
			}
		}
		input.RateLimits = rl
	}

	other := loadOtherAccount(cfgDir)
	return renderAndPrint(input, other, accountName, compact)
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

	// Sort account names for deterministic iteration order.
	names := make([]string, 0, len(reg.Accounts))
	for name := range reg.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)

	// Collect all other accounts with valid rate caches, then pick the one
	// with the highest 5h usage (most urgent to show as an alternative).
	type candidate struct {
		other    *statusline.OtherAccount
		fiveHour float64
	}
	var candidates []candidate

	normalizedCurrentDir := normalizePath(currentCfgDir)
	for _, name := range names {
		acc := reg.Accounts[name]
		accDir := acc.ConfigDir
		if accDir == "" {
			resolved, err := config.DefaultConfigDir()
			if err != nil {
				continue
			}
			accDir = resolved
		}
		if normalizePath(accDir) == normalizedCurrentDir {
			continue
		}

		cachePath := filepath.Join(accDir, "rate-cache.json")
		rc, err := cache.ReadRateCache(cachePath)
		if err != nil || rc == nil {
			continue
		}

		other := &statusline.OtherAccount{Name: name}
		var fiveHour float64

		if rc.RateLimits != nil {
			if rc.RateLimits.FiveHour != nil {
				fiveHour = rc.RateLimits.FiveHour.UsedPercentage
				other.FiveHour = fiveHour
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
			if age > statuslineStaleCacheThreshold {
				other.Stale = format.FormatDuration(age)
			}
		}

		candidates = append(candidates, candidate{other: other, fiveHour: fiveHour})
	}

	if len(candidates) == 0 {
		return nil
	}

	// Pick the candidate with the highest 5h usage. On tie, the earlier
	// name in sorted order wins (stable since names were sorted above).
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.fiveHour > best.fiveHour {
			best = c
		}
	}
	return best.other
}

// currentAccountName resolves the display name of the active account by
// matching cfgDir against the registry. Returns empty string on any error.
func currentAccountName(cfgDir string) string {
	regPath, err := config.RegistryPath()
	if err != nil {
		return ""
	}
	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		return ""
	}

	// The primary account may have an empty ConfigDir (uses default).
	// Use DefaultConfigDir (not DetectConfigDir) for empty config_dir to avoid
	// matching the CLAUDE_CONFIG_DIR override set by cx for the active session.
	normalizedCfg := normalizePath(cfgDir)
	for name, acc := range reg.Accounts {
		dir := acc.ConfigDir
		if dir == "" {
			resolved, err := config.DefaultConfigDir()
			if err != nil {
				continue
			}
			dir = resolved
		}
		if normalizePath(dir) == normalizedCfg {
			return name
		}
	}
	return ""
}

// normalizePath converts a path to a canonical form for comparison.
// On Windows, this lowercases and normalizes separators to forward slashes.
func normalizePath(p string) string {
	p = filepath.Clean(p)
	p = strings.ReplaceAll(p, "\\", "/")
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}

func renderAndPrint(input *statusline.Input, other *statusline.OtherAccount, accountName string, compact bool) error {
	opts := statusline.RenderOpts{
		AccountName: accountName,
		Compact:     compact,
	}

	// Load statusline config from registry.
	if regPath, err := config.RegistryPath(); err == nil {
		if reg, err := config.LoadOrCreateRegistry(regPath); err == nil && reg.Statusline != nil {
			sc := reg.Statusline
			opts.Sections = &statusline.SectionVisibility{
				ShowAccount:      sc.ShowAccount,
				ShowCost:         sc.ShowCost,
				ShowContext:      sc.ShowContext,
				ShowRate5h:       sc.ShowRate5h,
				ShowRate7d:       sc.ShowRate7d,
				ShowOtherAccount: sc.ShowOtherAccount,
				ShowSwitchHint:   sc.ShowSwitchHint,
			}
		}
	}

	lines := statusline.Render(input, other, platform.ANSIEnabled(), opts)
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

// withLogFile opens the cx.log file and passes a logger to fn.
// Silently returns if the config dir or log file cannot be resolved.
func withLogFile(fn func(*log.Logger)) {
	cfgDir, err := config.DetectConfigDir()
	if err != nil {
		return
	}
	logPath := filepath.Join(cfgDir, "cx.log")
	f, err := openLogFile(logPath)
	if err != nil {
		return
	}
	defer f.Close()
	fn(log.New(f, "", log.LstdFlags))
}

func logError(err error) {
	withLogFile(func(l *log.Logger) { l.Printf("ERROR: %v", err) })
}

func logWarn(msg string) {
	withLogFile(func(l *log.Logger) { l.Printf("[WARN] %s", msg) })
}
