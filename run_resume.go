package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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

	// Launch claude --resume with the correct account.
	fmt.Fprintf(os.Stderr, "[cx] Resuming %q on account %s\n", displaySlug(selected), selected.Account)

	env := replaceOrAppendEnv(os.Environ(), "CLAUDE_CONFIG_DIR", selected.ConfigDir)
	if err := platform.ExecProgram("claude", []string{"--resume", selected.ID}, env); err != nil {
		fmt.Fprintf(os.Stderr, "[cx] failed to exec claude: %v\n", err)
		os.Exit(1)
	}
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
