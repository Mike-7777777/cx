package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Mike-7777777/cc-monitor/internal/cache"
	"github.com/Mike-7777777/cc-monitor/internal/config"
	"github.com/Mike-7777777/cc-monitor/internal/platform"
)

const (
	preferThreshold  = 80.0 // --prefer: fall back if 5h usage >= this
	balanceThreshold = 90.0 // --balance: skip account if 5h usage > this
)

// accountScore pairs an account name with its effective 5h usage percentage.
type accountScore struct {
	name    string
	dir     string
	fiveHPct float64
}

func runRun() {
	args := os.Args[2:] // everything after "run"

	// Parse flags that belong to cc-monitor; the rest goes to claude.
	var preferName string
	var balance bool
	var claudeArgs []string

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--help" || args[i] == "-h":
			printRunHelp()
			return
		case args[i] == "--prefer" && i+1 < len(args):
			preferName = args[i+1]
			i++ // skip the value
		case strings.HasPrefix(args[i], "--prefer="):
			preferName = strings.TrimPrefix(args[i], "--prefer=")
		case args[i] == "--balance":
			balance = true
		case args[i] == "--":
			// Everything after "--" is forwarded literally to claude.
			claudeArgs = append(claudeArgs, args[i+1:]...)
			i = len(args) // break loop
		default:
			// First non-flag argument: treat the rest as claude args.
			claudeArgs = append(claudeArgs, args[i:]...)
			i = len(args) // break loop
		}
	}

	// Load registry.
	regPath, err := config.RegistryPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[cc-monitor] %v\n", err)
		os.Exit(1)
	}

	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[cc-monitor] %v\n", err)
		os.Exit(1)
	}

	if len(reg.Accounts) == 0 {
		fmt.Fprintln(os.Stderr, "[cc-monitor] No accounts configured. Run: cc-monitor init <name>")
		os.Exit(1)
	}

	// Score all accounts by 5h usage.
	scores := scoreAccounts(reg)
	if len(scores) == 0 {
		fmt.Fprintln(os.Stderr, "[cc-monitor] No accounts with rate data; picking first account")
		// Fall back to first account alphabetically.
		scores = fallbackScores(reg)
	}

	// Select account based on strategy.
	var selected accountScore
	var reason string

	switch {
	case preferName != "":
		selected, reason = selectPreferred(scores, preferName)
	case balance:
		selected, reason = selectBalanced(scores)
	default:
		selected, reason = selectLowest(scores)
	}

	fmt.Fprintf(os.Stderr, "[cc-monitor] Auto-selected: %s (5h: %.0f%%) %s\n",
		selected.name, selected.fiveHPct, reason)

	// Build env with CLAUDE_CONFIG_DIR set.
	env := replaceOrAppendEnv(os.Environ(), "CLAUDE_CONFIG_DIR", selected.dir)

	// Exec into claude.
	if err := platform.ExecProgram("claude", claudeArgs, env); err != nil {
		fmt.Fprintf(os.Stderr, "[cc-monitor] failed to exec claude: %v\n", err)
		os.Exit(1)
	}
}

// scoreAccounts reads the rate-cache for each account and returns scored entries
// sorted by 5h usage ascending.
func scoreAccounts(reg *config.Registry) []accountScore {
	var scores []accountScore

	for name, acc := range reg.Accounts {
		dir := acc.ConfigDir
		if dir == "" {
			d, err := config.DetectConfigDir()
			if err != nil {
				continue
			}
			dir = d
		}

		rc, err := cache.ReadRateCache(filepath.Join(dir, "rate-cache.json"))
		if err != nil || rc == nil || rc.RateLimits == nil || rc.RateLimits.FiveHour == nil {
			continue
		}

		pct := rc.RateLimits.FiveHour.UsedPercentage
		// If cache is stale but the window has reset, treat as 0%.
		if rc.RateLimits.FiveHour.IsReset() {
			pct = 0
		}

		scores = append(scores, accountScore{name: name, dir: dir, fiveHPct: pct})
	}

	sort.Slice(scores, func(i, j int) bool {
		if scores[i].fiveHPct != scores[j].fiveHPct {
			return scores[i].fiveHPct < scores[j].fiveHPct
		}
		return scores[i].name < scores[j].name
	})

	return scores
}

// fallbackScores returns all accounts with 0% score, sorted alphabetically.
func fallbackScores(reg *config.Registry) []accountScore {
	var scores []accountScore
	for name, acc := range reg.Accounts {
		dir := acc.ConfigDir
		if dir == "" {
			d, err := config.DetectConfigDir()
			if err != nil {
				continue
			}
			dir = d
		}
		scores = append(scores, accountScore{name: name, dir: dir, fiveHPct: 0})
	}
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].name < scores[j].name
	})
	return scores
}

// selectLowest picks the account with the lowest 5h usage.
func selectLowest(scores []accountScore) (accountScore, string) {
	return scores[0], ""
}

// selectPreferred picks the preferred account if its usage is below the
// threshold, otherwise falls back to the lowest.
func selectPreferred(scores []accountScore, prefer string) (accountScore, string) {
	for _, s := range scores {
		if s.name == prefer {
			if s.fiveHPct < preferThreshold {
				return s, fmt.Sprintf("[preferred, under %.0f%%]", preferThreshold)
			}
			// Preferred account is too hot; fall back.
			best := scores[0]
			return best, fmt.Sprintf("[preferred %q at %.0f%% >= %.0f%%, fell back]",
				prefer, s.fiveHPct, preferThreshold)
		}
	}
	// Preferred name not found; fall back to lowest.
	return scores[0], fmt.Sprintf("[preferred %q not found, fell back]", prefer)
}

// selectBalanced implements round-robin selection, skipping overloaded accounts.
func selectBalanced(scores []accountScore) (accountScore, string) {
	counter := readRunCounter()
	counter++

	n := len(scores)
	// Try round-robin, skipping accounts over the threshold.
	for attempt := 0; attempt < n; attempt++ {
		idx := (counter + attempt) % n
		if scores[idx].fiveHPct <= balanceThreshold {
			writeRunCounter(counter)
			return scores[idx], "[balanced]"
		}
	}

	// All accounts are over threshold; pick lowest anyway.
	writeRunCounter(counter)
	return scores[0], "[balanced, all above threshold]"
}

// runCounterPath returns ~/.cc-monitor-run-count.
func runCounterPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cc-monitor-run-count")
}

func readRunCounter() int {
	p := runCounterPath()
	if p == "" {
		return 0
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return n
}

func writeRunCounter(n int) {
	p := runCounterPath()
	if p == "" {
		return
	}
	_ = os.WriteFile(p, []byte(strconv.Itoa(n)+"\n"), 0o600)
}

// replaceOrAppendEnv sets key=value in the env slice, replacing an existing
// entry if present.
func replaceOrAppendEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func printRunHelp() {
	fmt.Print(`cc-monitor run — auto-select best account and launch claude

Usage:
  cc-monitor run [options] [-- claude-args...]

Options:
  --prefer <name>   Prefer a specific account (falls back if 5h usage >= 80%)
  --balance         Round-robin selection for maximum throughput
  --help, -h        Show this help

Examples:
  cc-monitor run                    # pick lowest-usage account
  cc-monitor run --prefer primary   # prefer "primary", fall back if hot
  cc-monitor run --balance          # alternate between accounts
  cc-monitor run -- -p "fix bug"   # pass args to claude after --
`)
}
