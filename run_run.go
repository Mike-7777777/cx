package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Mike-7777777/cx/internal/cache"
	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/format"
	"github.com/Mike-7777777/cx/internal/platform"
	"github.com/natefinch/atomic"
)

const (
	preferThreshold  = 80.0 // --prefer: fall back if 5h usage >= this
	balanceThreshold = 90.0 // --balance: skip account if 5h usage > this
)

// accountScore pairs an account name with its effective 5h usage and reset timing.
type accountScore struct {
	name        string
	dir         string
	fiveHPct    float64
	timeToReset time.Duration
}

// smartScore calculates a routing score. Lower = should use first.
// Accounts closer to reset with remaining capacity are preferred.
//
// Insight: if account A has 3h50m until reset (24% used) and account B has
// 48m until reset (71% used), B should be used first (it resets soon),
// saving A for after B resets.
func smartScore(usagePct float64, timeToReset time.Duration) float64 {
	const maxWindow = 5 * time.Hour

	// Normalize timeToReset to [0, 1] where 0 = about to reset, 1 = just started.
	resetFraction := float64(timeToReset) / float64(maxWindow)
	if resetFraction > 1 {
		resetFraction = 1
	}
	if resetFraction < 0 {
		resetFraction = 0
	}

	// If usage is very high (>90%), penalize heavily regardless of reset time.
	if usagePct > 90 {
		return 1000 + usagePct
	}

	// Score: prefer accounts that reset soon AND have remaining capacity.
	// Low resetFraction (about to reset) + moderate usage = good (low score).
	// High resetFraction (just started window) + low usage = save for later (higher score).
	return usagePct * resetFraction
}

func runRun() {
	args := os.Args[2:] // everything after "run"

	// Parse flags that belong to cx; the rest goes to claude.
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
		fmt.Fprintf(os.Stderr, "[cx] %v\n", err)
		os.Exit(1)
	}

	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[cx] %v\n", err)
		os.Exit(1)
	}

	if len(reg.Accounts) == 0 {
		fmt.Fprintln(os.Stderr, "[cx] No accounts configured. Run: cx init <name>")
		os.Exit(1)
	}

	// Score all accounts by smart routing (usage + time-to-reset).
	scores := scoreAccounts(reg)
	if len(scores) == 0 {
		fmt.Fprintln(os.Stderr, "[cx] No accounts with rate data; picking first account")
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

	if selected.timeToReset > 0 {
		fmt.Fprintf(os.Stderr, "[cx] Auto-selected: %s (5h: %.0f%%, resets in %s) %s\n",
			selected.name, selected.fiveHPct,
			format.FormatDuration(selected.timeToReset), reason)
	} else {
		fmt.Fprintf(os.Stderr, "[cx] Auto-selected: %s (5h: %.0f%%) %s\n",
			selected.name, selected.fiveHPct, reason)
	}

	// Build env with CLAUDE_CONFIG_DIR set.
	env := replaceOrAppendEnv(os.Environ(), "CLAUDE_CONFIG_DIR", selected.dir)

	// Check credentials locally; launch interactive login if needed.
	// We do NOT refresh tokens ourselves (Anthropic ToS prohibits third-party
	// tools from calling their OAuth endpoints). CC handles refresh internally.
	status := checkCredentials(selected.dir)
	if status != credentialOK {
		fmt.Fprintf(os.Stderr, "[cx] %s for %q — launching login...\n",
			credentialMessage(status), selected.name)
		if err := launchLogin(selected.dir); err != nil {
			fmt.Fprintf(os.Stderr, "[cx] login failed: %v\n", err)
			os.Exit(1)
		}
	}

	// Exec into claude.
	if err := platform.ExecProgram("claude", claudeArgs, env); err != nil {
		fmt.Fprintf(os.Stderr, "[cx] failed to exec claude: %v\n", err)
		os.Exit(1)
	}
}

// scoreAccounts reads the rate-cache for each account and returns scored entries
// sorted by smart score (usage weighted by time-to-reset).
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
		ttr := rc.RateLimits.FiveHour.TimeToReset()

		// If cache is stale but the window has reset, treat as 0%.
		if rc.RateLimits.FiveHour.IsReset() {
			pct = 0
			ttr = 0
		}

		scores = append(scores, accountScore{
			name:        name,
			dir:         dir,
			fiveHPct:    pct,
			timeToReset: ttr,
		})
	}

	sort.Slice(scores, func(i, j int) bool {
		si := smartScore(scores[i].fiveHPct, scores[i].timeToReset)
		sj := smartScore(scores[j].fiveHPct, scores[j].timeToReset)
		if si != sj {
			return si < sj
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

// selectLowest picks the account with the best smart score and shows reset info.
func selectLowest(scores []accountScore) (accountScore, string) {
	s := scores[0]
	if s.timeToReset > 0 {
		return s, fmt.Sprintf("[resets in %s]", format.FormatDuration(s.timeToReset))
	}
	return s, ""
}

// selectPreferred picks the preferred account if its usage is below the
// threshold, otherwise falls back to the best smart-scored account.
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
	// Preferred name not found; fall back to best smart-scored.
	return scores[0], fmt.Sprintf("[preferred %q not found, fell back]", prefer)
}

// selectBalanced implements round-robin selection, skipping overloaded accounts.
// Accounts are sorted by smart score, so round-robin operates over a
// schedule-aware ordering.
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

	// All accounts are over threshold; pick best smart-scored anyway.
	writeRunCounter(counter)
	return scores[0], "[balanced, all above threshold]"
}

// runCounterPath returns ~/.cx-run-count.
func runCounterPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cx-run-count")
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
	_ = atomic.WriteFile(p, bytes.NewReader([]byte(strconv.Itoa(n)+"\n")))
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
	fmt.Print(`cx run — auto-select best account and launch claude

Usage:
  cx run [options] [-- claude-args...]

Options:
  --prefer <name>   Prefer a specific account (falls back if 5h usage >= 80%)
  --balance         Round-robin selection for maximum throughput
  --help, -h        Show this help

Examples:
  cx run                    # pick lowest-usage account
  cx run --prefer primary   # prefer "primary", fall back if hot
  cx run --balance          # alternate between accounts
  cx run -- -p "fix bug"   # pass args to claude after --
`)
}
