package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/platform"
)

// resumeCmd implements Runner for the "resume" subcommand.
type resumeCmd struct{}

// resumeOpts holds parsed arguments for cx resume.
type resumeOpts struct {
	showHelp   bool
	last       bool
	on         string
	prefer     string
	searchTerm string
	claudeArgs []string
}

// parseResumeArgs splits args into cx-specific options, a fuzzy search term,
// and claude pass-through args.
//
// cx consumes: --help/-h, --last, --on, --prefer/-pf, --yolo/-y.
// The search term (if any) MUST be the first positional arg — this keeps
// parsing unambiguous when claude flags take values (e.g. `--model sonnet`).
// All other args (including unknown flags like --remote-control, --model)
// are forwarded to claude as-is, mirroring cx run's behaviour. A literal "--"
// separator forwards everything after it verbatim.
//
// --on and --prefer are mutually exclusive: --on forces an account (error if
// missing), --prefer selects an account with rate-limit fallback.
func parseResumeArgs(args []string) (opts resumeOpts, err error) {
	// First bare positional arg (if present before any flag) is the search term.
	start := 0
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		opts.searchTerm = args[0]
		start = 1
	}

	for i := start; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--help" || a == "-h":
			opts.showHelp = true
			return
		case a == "--last":
			opts.last = true
		case a == "--on":
			if i+1 >= len(args) {
				err = fmt.Errorf("--on requires an account name")
				return
			}
			opts.on = args[i+1]
			i++
		case strings.HasPrefix(a, "--on="):
			opts.on = strings.TrimPrefix(a, "--on=")
			if opts.on == "" {
				err = fmt.Errorf("--on requires an account name")
				return
			}
		case a == "--prefer" || a == "-pf":
			if i+1 >= len(args) {
				err = fmt.Errorf("%s requires an account name", a)
				return
			}
			opts.prefer = args[i+1]
			i++
		case strings.HasPrefix(a, "--prefer="):
			opts.prefer = strings.TrimPrefix(a, "--prefer=")
			if opts.prefer == "" {
				err = fmt.Errorf("--prefer requires an account name")
				return
			}
		case a == "--":
			opts.claudeArgs = append(opts.claudeArgs, args[i+1:]...)
			return
		default:
			if mapped, ok := runAliases[a]; ok {
				opts.claudeArgs = append(opts.claudeArgs, mapped)
				continue
			}
			opts.claudeArgs = append(opts.claudeArgs, a)
		}
	}
	if opts.on != "" && opts.prefer != "" {
		err = fmt.Errorf("--on and --prefer are mutually exclusive")
		return
	}
	return
}

const resumeHelpText = `cx resume — resume a CC session with smart matching

Usage:
  cx resume [options] [term] [claude-flags...]

cx-specific options:
  --last                   Resume the most recent session (any account)
  --on <acct>              Force a specific account (error if unknown)
  -pf, --prefer <acct>     Prefer an account (falls back if 5h usage >= 80%)
  -y, --yolo               Alias for --dangerously-skip-permissions
  -h, --help               Show this help

[term] (optional, must be FIRST) fuzzy-matches against session slug,
project, account, or ID prefix. Putting the term first avoids ambiguity
with claude flags that take values (e.g. --model sonnet).
All other flags are forwarded to claude (e.g. --remote-control, --model).

Examples:
  cx resume                           # Interactive picker
  cx resume fix-bug                   # Match session by slug
  cx resume --last --prefer QM        # Most recent session, prefer QM
  cx resume -y --remote-control       # Yolo + remote-control
  cx resume -- -p "continue task"     # Explicit separator for claude args
`

// Run resumes a CC session with smart matching.
func (c *resumeCmd) Run(_ context.Context, app *App, args []string) error {
	opts, err := parseResumeArgs(args)
	if err != nil {
		return err
	}
	if opts.showHelp {
		fmt.Fprint(app.Stdout, resumeHelpText)
		return nil
	}

	// Collect every session across every account. No limit: the picker
	// needs the full list so users can pick older sessions, and fuzzy
	// search benefits from matching across the whole history.
	sessions := collectSessions(app.Registry, "", 0)
	if len(sessions) == 0 {
		return fmt.Errorf("no sessions found")
	}

	var selected *sessionEntry

	switch {
	case opts.last:
		selected = &sessions[0]
	case opts.searchTerm != "":
		var matches []sessionEntry
		term := strings.ToLower(opts.searchTerm)
		for _, s := range sessions {
			if strings.Contains(strings.ToLower(s.Slug), term) ||
				strings.Contains(strings.ToLower(s.Project), term) ||
				strings.Contains(strings.ToLower(s.Account), term) ||
				strings.HasPrefix(s.ID, opts.searchTerm) {
				matches = append(matches, s)
			}
		}

		if len(matches) == 0 {
			return fmt.Errorf("no session matching %q", opts.searchTerm)
		}
		if len(matches) == 1 {
			selected = &matches[0]
		} else {
			selected = pickSession(matches, app.Registry, app.Stderr)
		}
	default:
		selected = pickSession(sessions, app.Registry, app.Stderr)
	}

	if selected == nil {
		return nil
	}

	// Determine which account to run on.
	configDir := selected.ConfigDir
	accountName := selected.Account
	var preferReason string

	switch {
	case opts.on != "":
		if app.Registry != nil {
			dir, err := app.Registry.ResolveConfigDir(opts.on)
			if err != nil {
				return fmt.Errorf("unknown account %q", opts.on)
			}
			configDir = dir
			accountName = opts.on
		}
	case opts.prefer != "":
		if app.Registry != nil {
			scores := scoreAccounts(app.Registry)
			scores = ensureAllAccounts(app.Registry, scores)
			if len(scores) == 0 {
				return fmt.Errorf("no accounts available")
			}
			chosen, reason := selectPreferred(scores, opts.prefer)
			configDir = chosen.dir
			accountName = chosen.name
			preferReason = reason
		}
	default:
		// No explicit account choice — ask interactively when ambiguous.
		if app.Registry != nil && len(app.Registry.Accounts) > 1 {
			configDir, accountName = pickAccount(app.Registry, selected.Account, app.Stderr)
		}
	}

	// Cross-account resume: ensure the session file exists in the target
	// account's projects directory. CC looks for the JSONL file relative
	// to CLAUDE_CONFIG_DIR, so we symlink/copy it if resuming on a
	// different account.
	if configDir != selected.ConfigDir && selected.ProjectDir != "" {
		srcFile := filepath.Join(selected.ConfigDir, "projects", selected.ProjectDir, selected.ID+".jsonl")
		dstDir := filepath.Join(configDir, "projects", selected.ProjectDir)
		dstFile := filepath.Join(dstDir, selected.ID+".jsonl")

		if _, err := os.Stat(dstFile); os.IsNotExist(err) {
			_ = os.MkdirAll(dstDir, 0o755)
			// Symlink is preferred (no data duplication); fall back to copy.
			if linkErr := os.Symlink(srcFile, dstFile); linkErr != nil {
				data, readErr := os.ReadFile(srcFile)
				if readErr != nil {
					return fmt.Errorf("cannot read session file: %v", readErr)
				}
				if writeErr := os.WriteFile(dstFile, data, 0o644); writeErr != nil {
					return fmt.Errorf("cannot copy session file: %v", writeErr)
				}
			}
			fmt.Fprintf(app.Stderr, "[cx] Linked session to %s\n", accountName)
		}
	}

	if preferReason != "" {
		fmt.Fprintf(app.Stderr, "[cx] Resuming %q on account %s %s\n",
			displaySlug(selected), accountName, preferReason)
	} else {
		fmt.Fprintf(app.Stderr, "[cx] Resuming %q on account %s\n",
			displaySlug(selected), accountName)
	}

	env := replaceOrAppendEnv(os.Environ(), "CLAUDE_CONFIG_DIR", configDir)
	claudeArgs := append([]string{"--resume", selected.ID}, opts.claudeArgs...)
	if err := platform.ExecProgram("claude", claudeArgs, env); err != nil {
		return fmt.Errorf("failed to exec claude: %v", err)
	}
	return nil
}

// pickAccount asks the user which account to run the session on.
// Returns the config dir and account name.
func pickAccount(reg *config.Registry, defaultAccount string, w io.Writer) (string, string) {
	names := sortedAccountNames(reg)
	if len(names) <= 1 {
		dir, _ := reg.ResolveConfigDir(defaultAccount)
		return dir, defaultAccount
	}

	fmt.Fprintf(w, "\n  Run on which account? ")
	for i, name := range names {
		marker := ""
		if name == defaultAccount {
			marker = "*"
		}
		if i > 0 {
			fmt.Fprint(w, " / ")
		}
		fmt.Fprintf(w, "%s%s", name, marker)
	}
	fmt.Fprintf(w, " [%s]: ", defaultAccount)

	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)

	if line == "" {
		line = defaultAccount
	}

	dir, err := reg.ResolveConfigDir(line)
	if err != nil {
		fmt.Fprintf(w, "[cx] unknown account %q, using %s\n", line, defaultAccount)
		dir, _ = reg.ResolveConfigDir(defaultAccount)
		return dir, defaultAccount
	}
	return dir, line
}

// pickSession shows a numbered list and reads user choice from stdin.
// Every collected session is listed — no silent truncation — so the user
// can reach older work. Narrow the list with a fuzzy `cx resume <term>`
// when history grows large.
func pickSession(sessions []sessionEntry, reg *config.Registry, w io.Writer) *sessionEntry {

	max := len(sessions)

	fmt.Fprintln(w)
	fmt.Fprintf(w, "       %-4s  %-8s  %s\n", "Acct", "Age", "Topic")
	fmt.Fprintf(w, "      %s\n", strings.Repeat("─", 70))
	for i, s := range sessions[:max] {
		marker := "  "
		if reg != nil && s.Account == reg.Main {
			marker = "★ "
		}

		activeTag := ""
		if s.Active {
			activeTag = "[active] "
		}

		topic := sessionTopic(&s)
		fmt.Fprintf(w, "  %s%2d) %-4s  %s%-8s  %s\n",
			marker, i+1, s.Account, activeTag, formatAge(s.Age), topic)
	}

	fmt.Fprintf(w, "\n  Pick [1-%d]: ", max)

	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)

	if line == "" || line == "q" {
		return nil
	}

	num := 0
	for _, c := range line {
		if c < '0' || c > '9' {
			fmt.Fprintln(w, "Invalid choice.")
			return nil
		}
		num = num*10 + int(c-'0')
	}

	if num < 1 || num > max {
		fmt.Fprintln(w, "Out of range.")
		return nil
	}

	return &sessions[num-1]
}

func displaySlug(s *sessionEntry) string {
	if s.Slug != "" {
		return s.Slug
	}
	if len(s.ID) > 12 {
		return s.ID[:12] + "..."
	}
	return s.ID
}
