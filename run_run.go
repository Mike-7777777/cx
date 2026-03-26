package main

import (
	"bytes"
	"context"
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

// runCmd implements Runner for the "run" subcommand.
type runCmd struct{}

const (
	preferThreshold  = 80.0 // --prefer: fall back if 5h usage >= this
	balanceThreshold = 90.0 // --balance: skip account if 5h usage > this
)

// accountScore pairs an account name with its effective 5h usage, reset timing,
// and 7d headroom for tiebreaking.
type accountScore struct {
	name           string
	dir            string
	fiveHPct       float64
	timeToReset    time.Duration
	sevenDHeadroom float64 // 7d headroom: remaining daily budget / normal daily budget (0-N, >1 = comfortable)
}

// smartScore calculates a routing score. Lower = should use first.
// Accounts closer to reset with remaining capacity are preferred.
//
// Insight: if account A has 3h50m until reset (24% used) and account B has
// 48m until reset (71% used), B should be used first (it resets soon),
// saving A for after B resets.
//
// The 7d headroom is used as a small tiebreaker: when two accounts have
// similar 5h scores, prefer the one with more 7d headroom (more remaining
// daily budget relative to time left in the week).
func smartScore(usagePct float64, timeToReset time.Duration, sevenDHeadroom float64) float64 {
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

	// Primary score from 5h window.
	score := usagePct * resetFraction

	// 7d tiebreaker: subtract a small bonus for high headroom (max 5 points).
	// headroom=1.0 (on budget) → bonus=0; headroom=2.0 (lots of room) → bonus=-5.
	// This ensures 7d only matters when 5h scores are close.
	if sevenDHeadroom > 0 {
		bonus := (sevenDHeadroom - 1.0) * 5
		if bonus > 5 {
			bonus = 5
		}
		score -= bonus
	}

	return score
}

// sevenDayHeadroom calculates how much daily budget remains relative to
// a normal daily budget (100%/7 ≈ 14.3%). Returns >1 if comfortable, <1 if tight.
// Example: 91% used, 1.5 days left → (9/1.5)/14.3 = 0.42 (tight).
// Example: 50% used, 4 days left → (50/4)/14.3 = 0.87 (slightly tight).
// Example: 30% used, 5 days left → (70/5)/14.3 = 0.98 (on track).
func sevenDayHeadroom(usedPct float64, daysLeft float64) float64 {
	if daysLeft <= 0 {
		return 1.0 // about to reset, headroom doesn't matter
	}
	remaining := 100.0 - usedPct
	dailyAvailable := remaining / daysLeft
	dailyBudget := 100.0 / 7.0
	return dailyAvailable / dailyBudget
}

// Run auto-selects the best account by smart routing and launches claude.
func (c *runCmd) Run(_ context.Context, app *App, args []string) error {
	w := app.Stderr
	reg := app.Registry

	// Parse flags that belong to cx; the rest goes to claude.
	var preferName string
	var balance bool
	var yolo bool
	var claudeArgs []string

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--help" || args[i] == "-h":
			fmt.Fprint(app.Stdout, runHelpText)
			return nil
		case args[i] == "--prefer" && i+1 < len(args):
			preferName = args[i+1]
			i++ // skip the value
		case strings.HasPrefix(args[i], "--prefer="):
			preferName = strings.TrimPrefix(args[i], "--prefer=")
		case args[i] == "--balance":
			balance = true
		case args[i] == "--yolo":
			yolo = true
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

	if yolo {
		claudeArgs = append(claudeArgs, "--dangerously-skip-permissions")
	}

	if len(reg.Accounts) == 0 {
		return fmt.Errorf("no accounts configured. Run: cx init <name>")
	}

	// Score all accounts by smart routing (usage + time-to-reset).
	// Accounts without rate-cache data get 0% usage (treated as fully available).
	scores := scoreAccounts(reg)
	if len(scores) == 0 {
		fmt.Fprintln(w, "[cx] No accounts with rate data; picking first account")
	}
	// Ensure ALL registered accounts are in the list (with 0% if no data).
	scores = ensureAllAccounts(reg, scores)

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
		fmt.Fprintf(w, "[cx] Auto-selected: %s (5h: %.0f%%, resets in %s) %s\n",
			selected.name, selected.fiveHPct,
			format.FormatDuration(selected.timeToReset), reason)
	} else {
		fmt.Fprintf(w, "[cx] Auto-selected: %s (5h: %.0f%%) %s\n",
			selected.name, selected.fiveHPct, reason)
	}

	// Build env with CLAUDE_CONFIG_DIR set.
	env := replaceOrAppendEnv(os.Environ(), "CLAUDE_CONFIG_DIR", selected.dir)

	// Check credentials locally; launch interactive login if needed.
	// We do NOT refresh tokens ourselves (Anthropic ToS prohibits third-party
	// tools from calling their OAuth endpoints). CC handles refresh internally.
	status := checkCredentials(selected.dir)
	if status != credentialOK {
		fmt.Fprintf(w, "[cx] %s for %q — launching login...\n",
			credentialMessage(status), selected.name)
		if err := launchLogin(selected.dir); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
	}

	// Exec into claude.
	if err := platform.ExecProgram("claude", claudeArgs, env); err != nil {
		return fmt.Errorf("failed to exec claude: %w", err)
	}
	return nil
}

// scoreAccounts reads the rate-cache for each account and returns scored entries
// sorted by smart score (usage weighted by time-to-reset).
func scoreAccounts(reg *config.Registry) []accountScore {
	var scores []accountScore

	for name := range reg.Accounts {
		dir, err := reg.ResolveConfigDir(name)
		if err != nil {
			continue
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

		// Calculate 7d headroom for tiebreaking.
		var headroom float64 = 1.0 // default: neutral
		if rc.RateLimits.SevenDay != nil && !rc.RateLimits.SevenDay.IsReset() {
			daysLeft := time.Until(time.Unix(rc.RateLimits.SevenDay.ResetsAt, 0)).Hours() / 24
			headroom = sevenDayHeadroom(rc.RateLimits.SevenDay.UsedPercentage, daysLeft)
		}

		scores = append(scores, accountScore{
			name:           name,
			dir:            dir,
			fiveHPct:       pct,
			timeToReset:    ttr,
			sevenDHeadroom: headroom,
		})
	}

	sort.Slice(scores, func(i, j int) bool {
		si := smartScore(scores[i].fiveHPct, scores[i].timeToReset, scores[i].sevenDHeadroom)
		sj := smartScore(scores[j].fiveHPct, scores[j].timeToReset, scores[j].sevenDHeadroom)
		if si != sj {
			return si < sj
		}
		return scores[i].name < scores[j].name
	})

	return scores
}

// ensureAllAccounts adds any registered accounts missing from scores with 0%.
// This guarantees --prefer can always find the requested account.
func ensureAllAccounts(reg *config.Registry, scores []accountScore) []accountScore {
	have := make(map[string]bool, len(scores))
	for _, s := range scores {
		have[s.name] = true
	}
	for name := range reg.Accounts {
		if have[name] {
			continue
		}
		dir, err := reg.ResolveConfigDir(name)
		if err != nil {
			continue
		}
		scores = append(scores, accountScore{name: name, dir: dir, fiveHPct: 0})
	}
	sort.Slice(scores, func(i, j int) bool {
		si := smartScore(scores[i].fiveHPct, scores[i].timeToReset, scores[i].sevenDHeadroom)
		sj := smartScore(scores[j].fiveHPct, scores[j].timeToReset, scores[j].sevenDHeadroom)
		if si != sj {
			return si < sj
		}
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

const runHelpText = `cx run — auto-select best account and launch claude

Usage:
  cx run [options] [-- claude-args...]

Options:
  --yolo            Launch with --dangerously-skip-permissions
  --prefer <name>   Prefer a specific account (falls back if 5h usage >= 80%)
  --balance         Round-robin selection for maximum throughput
  --help, -h        Show this help

Examples:
  cx run                    # pick lowest-usage account
  cx run --yolo             # skip permissions (same as cx run -- --dangerously-skip-permissions)
  cx run --prefer work      # prefer "work", fall back if hot
  cx run --balance          # alternate between accounts
  cx run -- -p "fix bug"   # pass args to claude after --
`
