package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/platform"
)

func runResume() {
	args := os.Args[2:]

	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(`cx resume — resume a CC session with smart matching

Usage:
  cx resume              Interactive picker (numbered list)
  cx resume <term>       Fuzzy match by slug, project, or account name
  cx resume --last       Resume the most recent session (any account)
  cx resume --on <acct>  Run session on a specific account (cross-account resume)
`)
		return
	}

	isLast := len(args) > 0 && args[0] == "--last"
	searchTerm := ""
	if len(args) > 0 && !isLast {
		searchTerm = args[0]
	}

	// Collect sessions.
	sessions := collectSessions("", 50)
	if len(sessions) == 0 {
		fmt.Fprintln(os.Stderr, "No sessions found.")
		os.Exit(1)
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
			fmt.Fprintf(os.Stderr, "No session matching %q\n", searchTerm)
			os.Exit(1)
		}
		if len(matches) == 1 {
			selected = &matches[0]
		} else {
			// Multiple matches — show picker.
			selected = pickSession(matches)
		}
	} else {
		// No args — interactive picker.
		selected = pickSession(sessions)
	}

	if selected == nil {
		return
	}

	// Determine which account to run on.
	configDir := selected.ConfigDir
	accountName := selected.Account

	// If --on <account> flag is given, use that account instead.
	onAccount := ""
	for i := 2; i < len(os.Args); i++ {
		if os.Args[i] == "--on" && i+1 < len(os.Args) {
			onAccount = os.Args[i+1]
			break
		}
		if strings.HasPrefix(os.Args[i], "--on=") {
			onAccount = strings.TrimPrefix(os.Args[i], "--on=")
			break
		}
	}

	if onAccount == "" {
		// Interactive: if multiple accounts exist, ask which one to use.
		reg := loadRegistryOrNil()
		if reg != nil && len(reg.Accounts) > 1 {
			configDir, accountName = pickAccount(reg, selected.Account)
		}
	} else {
		reg := loadRegistryOrNil()
		if reg != nil {
			dir, err := reg.ResolveConfigDir(onAccount)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[cx] unknown account %q\n", onAccount)
				os.Exit(1)
			}
			configDir = dir
			accountName = onAccount
		}
	}

	fmt.Fprintf(os.Stderr, "[cx] Resuming %q on account %s\n", displaySlug(selected), accountName)

	env := replaceOrAppendEnv(os.Environ(), "CLAUDE_CONFIG_DIR", configDir)
	if err := platform.ExecProgram("claude", []string{"--resume", selected.ID}, env); err != nil {
		fmt.Fprintf(os.Stderr, "[cx] failed to exec claude: %v\n", err)
		os.Exit(1)
	}
}

// pickAccount asks the user which account to run the session on.
// Returns the config dir and account name.
func pickAccount(reg *config.Registry, defaultAccount string) (string, string) {
	names := sortedNames(reg)
	if len(names) <= 1 {
		dir, _ := reg.ResolveConfigDir(defaultAccount)
		return dir, defaultAccount
	}

	fmt.Fprintf(os.Stderr, "\n  Run on which account? ")
	for i, name := range names {
		marker := ""
		if name == defaultAccount {
			marker = "*"
		}
		if i > 0 {
			fmt.Fprint(os.Stderr, " / ")
		}
		fmt.Fprintf(os.Stderr, "%s%s", name, marker)
	}
	fmt.Fprintf(os.Stderr, " [%s]: ", defaultAccount)

	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)

	if line == "" {
		line = defaultAccount
	}

	dir, err := reg.ResolveConfigDir(line)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[cx] unknown account %q, using %s\n", line, defaultAccount)
		dir, _ = reg.ResolveConfigDir(defaultAccount)
		return dir, defaultAccount
	}
	return dir, line
}

// pickSession shows a numbered list and reads user choice from stdin.
func pickSession(sessions []sessionEntry) *sessionEntry {
	useColor := platform.ANSIEnabled()
	reg := loadRegistryOrNil()

	max := len(sessions)
	if max > 15 {
		max = 15
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "       %-4s  %-8s  %s\n", "Acct", "Age", "Topic")
	fmt.Fprintf(os.Stderr, "      %s\n", strings.Repeat("─", 70))
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
		fmt.Fprintf(os.Stderr, "  %s%2d) %-4s  %s%-8s  %s\n",
			marker, i+1, s.Account, activeTag, formatAge(s.Age), topic)
	}

	fmt.Fprintf(os.Stderr, "\n")
	if useColor {
		fmt.Fprint(os.Stderr, "  Pick [1-", max, "]: ")
	} else {
		fmt.Fprintf(os.Stderr, "  Pick [1-%d]: ", max)
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
			fmt.Fprintln(os.Stderr, "Invalid choice.")
			return nil
		}
		num = num*10 + int(c-'0')
	}

	if num < 1 || num > max {
		fmt.Fprintln(os.Stderr, "Out of range.")
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
