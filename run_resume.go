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

// Run resumes a CC session with smart matching.
func (c *resumeCmd) Run(_ context.Context, app *App, args []string) error {
	flags, positional := parseFlags(args, "last", "on", "yolo")

	// Check for help flag.
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Fprint(app.Stdout, `cx resume — resume a CC session with smart matching

Usage:
  cx resume              Interactive picker (numbered list)
  cx resume <term>       Fuzzy match by slug, project, or account name
  cx resume --last       Resume the most recent session (any account)
  cx resume --on <acct>  Run session on a specific account (cross-account resume)
  cx resume --yolo       Resume with --dangerously-skip-permissions
`)
			return nil
		}
	}

	_, isLast := flags["last"]
	onAccount := flags["on"]
	_, isYolo := flags["yolo"]

	searchTerm := ""
	if len(positional) > 0 {
		searchTerm = positional[0]
	}

	// Collect sessions.
	sessions := collectSessions("", 50)
	if len(sessions) == 0 {
		return fmt.Errorf("no sessions found")
	}

	var selected *sessionEntry

	if isLast {
		selected = &sessions[0]
	} else if searchTerm != "" {
		// Fuzzy match against slug, project, account, or ID.
		var matches []sessionEntry
		term := strings.ToLower(searchTerm)
		for _, s := range sessions {
			if strings.Contains(strings.ToLower(s.Slug), term) ||
				strings.Contains(strings.ToLower(s.Project), term) ||
				strings.Contains(strings.ToLower(s.Account), term) ||
				strings.HasPrefix(s.ID, searchTerm) {
				matches = append(matches, s)
			}
		}

		if len(matches) == 0 {
			return fmt.Errorf("no session matching %q", searchTerm)
		}
		if len(matches) == 1 {
			selected = &matches[0]
		} else {
			// Multiple matches — show picker.
			selected = pickSession(sessions, app.UseColor, app.Stderr)
		}
	} else {
		// No args — interactive picker.
		selected = pickSession(sessions, app.UseColor, app.Stderr)
	}

	if selected == nil {
		return nil
	}

	// Determine which account to run on.
	configDir := selected.ConfigDir
	accountName := selected.Account

	if onAccount == "" || onAccount == "true" {
		// Interactive: if multiple accounts exist, ask which one to use.
		reg := loadRegistryOrNil()
		if reg != nil && len(reg.Accounts) > 1 && onAccount != "true" {
			configDir, accountName = pickAccount(reg, selected.Account, app.Stderr)
		}
	} else {
		reg := loadRegistryOrNil()
		if reg != nil {
			dir, err := reg.ResolveConfigDir(onAccount)
			if err != nil {
				return fmt.Errorf("unknown account %q", onAccount)
			}
			configDir = dir
			accountName = onAccount
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

	fmt.Fprintf(app.Stderr, "[cx] Resuming %q on account %s\n", displaySlug(selected), accountName)

	env := replaceOrAppendEnv(os.Environ(), "CLAUDE_CONFIG_DIR", configDir)
	claudeArgs := []string{"--resume", selected.ID}
	if isYolo {
		claudeArgs = append(claudeArgs, "--dangerously-skip-permissions")
	}
	if err := platform.ExecProgram("claude", claudeArgs, env); err != nil {
		return fmt.Errorf("failed to exec claude: %v", err)
	}
	return nil
}

// pickAccount asks the user which account to run the session on.
// Returns the config dir and account name.
func pickAccount(reg *config.Registry, defaultAccount string, w io.Writer) (string, string) {
	names := sortedNames(reg)
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
func pickSession(sessions []sessionEntry, useColor bool, w io.Writer) *sessionEntry {
	reg := loadRegistryOrNil()

	max := len(sessions)
	if max > 15 {
		max = 15
	}

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

	fmt.Fprintf(w, "\n")
	if useColor {
		fmt.Fprint(w, "  Pick [1-", max, "]: ")
	} else {
		fmt.Fprintf(w, "  Pick [1-%d]: ", max)
	}

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
