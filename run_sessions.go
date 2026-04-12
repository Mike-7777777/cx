package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/format"
)

const defaultSessionLimit = 10

// sessionEntry holds metadata for one CC session across any account.
type sessionEntry struct {
	ID         string        `json:"id"`
	Account    string        `json:"account"`
	ConfigDir  string        `json:"-"`
	ProjectDir string        `json:"-"` // raw project directory name (e.g., "D--projects-myapp")
	Slug       string        `json:"slug,omitempty"`
	Project    string        `json:"project"`
	Model      string        `json:"model"`
	Age        time.Duration `json:"age_seconds"`
	Tokens     int64         `json:"tokens"`
	Active     bool          `json:"active"`
	FirstMsg   string        `json:"first_msg,omitempty"`
	LastMsg    string        `json:"last_msg,omitempty"`
}

// sessionsCmd implements Runner for the "sessions" subcommand.
type sessionsCmd struct{}

// Run lists recent CC sessions across all accounts.
func (c *sessionsCmd) Run(_ context.Context, app *App, args []string) error {
	flags, _ := parseFlags(args, "all", "json", "account")

	// Check for help flag.
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Fprint(app.Stdout, `cx sessions — list recent CC sessions across all accounts

Usage:
  cx sessions [options]

Options:
  --all              Show all sessions (default: last 10)
  --json             JSON output
  --account <name>   Filter by account
`)
			return nil
		}
	}

	_, showAll := flags["all"]
	_, jsonOut := flags["json"]
	accountFilter := flags["account"]

	limit := defaultSessionLimit
	if showAll {
		limit = 0
	}

	sessions := collectSessions(app.Registry, accountFilter, limit)
	if len(sessions) == 0 {
		fmt.Fprintln(app.Stdout, "No sessions found.")
		return nil
	}

	if jsonOut {
		data, _ := json.MarshalIndent(sessions, "", "  ")
		fmt.Fprintln(app.Stdout, string(data))
		return nil
	}

	printSessionTable(app, sessions, app.Registry, app.UseColor)
	return nil
}

// collectSessions gathers session metadata from all accounts.
func collectSessions(reg *config.Registry, accountFilter string, limit int) []sessionEntry {
	if reg == nil {
		return nil
	}

	// Find active session PIDs.
	activeSessions := make(map[string]bool)
	for _, name := range sortedAccountNames(reg) {
		dir, err := reg.ResolveConfigDir(name)
		if err != nil {
			continue
		}
		sessDir := filepath.Join(dir, "sessions")
		entries, _ := os.ReadDir(sessDir)
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, _ := os.ReadFile(filepath.Join(sessDir, e.Name()))
			var info struct {
				SessionID string `json:"sessionId"`
			}
			if json.Unmarshal(data, &info) == nil && info.SessionID != "" {
				activeSessions[info.SessionID] = true
			}
		}
	}

	var all []sessionEntry

	for _, name := range sortedAccountNames(reg) {
		if accountFilter != "" && name != accountFilter {
			continue
		}
		dir, err := reg.ResolveConfigDir(name)
		if err != nil {
			continue
		}

		projectsDir := filepath.Join(dir, "projects")
		projects, _ := os.ReadDir(projectsDir)

		for _, proj := range projects {
			if !proj.IsDir() {
				continue
			}
			projPath := filepath.Join(projectsDir, proj.Name())
			files, _ := os.ReadDir(projPath)

			for _, f := range files {
				if !strings.HasSuffix(f.Name(), ".jsonl") || f.IsDir() {
					continue
				}

				info, _ := f.Info()
				if info == nil {
					continue
				}

				sessionID := strings.TrimSuffix(f.Name(), ".jsonl")
				si := sessionEntry{
					ID:         sessionID,
					Account:    name,
					ConfigDir:  dir,
					ProjectDir: proj.Name(),
					Project:    shortProjectName(proj.Name()),
					Age:        time.Since(info.ModTime()),
					Active:     activeSessions[sessionID],
				}

				// Read first line for model, last lines for slug/tokens.
				readSessionMeta(filepath.Join(projPath, f.Name()), &si)
				all = append(all, si)
			}
		}
	}

	// Deduplicate sessions that appear in multiple accounts (cross-account symlinks).
	all = deduplicateSessions(all)

	// Sort by most recent first.
	sort.Slice(all, func(i, j int) bool {
		return all[i].Age < all[j].Age
	})

	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}

	return all
}

// deduplicateSessions removes duplicate session IDs, keeping the entry
// with the smallest Age (most recently modified) for each ID.
func deduplicateSessions(sessions []sessionEntry) []sessionEntry {
	best := make(map[string]int) // session ID -> index in result
	var result []sessionEntry

	for _, s := range sessions {
		if idx, exists := best[s.ID]; exists {
			if s.Age < result[idx].Age {
				result[idx] = s
			}
		} else {
			best[s.ID] = len(result)
			result = append(result, s)
		}
	}
	return result
}

// readSessionMeta extracts model, slug, and first/last user messages.
// Reads first 20 lines (model + first message) + last 8KB (slug + last message).
func readSessionMeta(path string, si *sessionEntry) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	// Read first 20 lines to find model and first user message.
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024) // handle large lines
	for i := 0; i < 20 && scanner.Scan(); i++ {
		line := scanner.Bytes()
		si.Model, si.FirstMsg = extractHeadMeta(line, si.Model, si.FirstMsg)
		if si.Model != "" && si.FirstMsg != "" {
			break
		}
	}

	// Read last 8KB to find slug and last user message.
	info, err := f.Stat()
	if err != nil {
		return
	}
	tailSize := int64(8192)
	if info.Size() < tailSize {
		tailSize = info.Size()
	}
	if _, err := f.Seek(-tailSize, 2); err != nil {
		return
	}
	tail, err := io.ReadAll(f)
	if err != nil {
		return
	}
	lines := strings.Split(string(tail), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if si.Slug == "" || si.LastMsg == "" {
			extractTailMeta(lines[i], si)
		} else {
			break
		}
	}

	// Estimate tokens from file size (~1 token per 15 bytes of JSONL).
	si.Tokens = info.Size() / 15
}

// extractHeadMeta tries to extract model and first user message from a line.
func extractHeadMeta(line []byte, curModel, curFirstMsg string) (string, string) {
	var entry struct {
		Type    string `json:"type"`
		Message struct {
			Model   string          `json:"model"`
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(line, &entry) != nil {
		return curModel, curFirstMsg
	}
	if curModel == "" && entry.Type == "assistant" && entry.Message.Model != "" {
		curModel = shortModelName(entry.Message.Model)
	}
	if curFirstMsg == "" && entry.Type == "user" {
		curFirstMsg = extractUserText(entry.Message.Content)
	}
	return curModel, curFirstMsg
}

// extractTailMeta extracts slug and last user message from a tail line.
func extractTailMeta(line string, si *sessionEntry) {
	var entry struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
		Slug    string `json:"slug"`
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal([]byte(line), &entry) != nil {
		return
	}
	if si.Slug == "" && entry.Type == "system" && entry.Subtype == "turn_duration" && entry.Slug != "" {
		si.Slug = entry.Slug
	}
	if si.LastMsg == "" && entry.Type == "user" {
		si.LastMsg = extractUserText(entry.Message.Content)
	}
}

// extractUserText extracts display text from a user message content field.
// Content can be a string or an array of content blocks [{type:"text",text:"..."}].
func extractUserText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	// Try string first.
	var s string
	if json.Unmarshal(raw, &s) == nil && s != "" {
		return truncateMsg(s)
	}

	// Try array of content blocks.
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) == nil {
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				return truncateMsg(b.Text)
			}
		}
	}

	return ""
}

// truncateMsg shortens a message, stripping newlines and CC internal tags.
func truncateMsg(s string) string {
	const maxLen = 40
	// Strip CC internal XML-like tags (e.g., <command-message>, <local-command-caveat>).
	s = stripTags(s)
	// Replace newlines with spaces for single-line display.
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "..."
	}
	return s
}

// stripTags removes XML/HTML-like tags from s.
func stripTags(s string) string {
	var out strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' && inTag {
			inTag = false
			continue
		}
		if !inTag {
			out.WriteRune(r)
		}
	}
	return out.String()
}

// shortProjectName converts a project slug to a short display name.
// "D--projects-myapp" → "homebase"
func shortProjectName(slug string) string {
	parts := strings.Split(slug, "-")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return slug
}

func printSessionTable(app *App, sessions []sessionEntry, reg *config.Registry, useColor bool) {
	header := fmt.Sprintf("  %-4s  %-8s  %s",
		"Acct", "Age", "Topic")
	sep := strings.Repeat("━", 80)

	fmt.Fprintln(app.Stdout, format.Colorize(header, format.Bold, useColor))
	fmt.Fprintln(app.Stdout, format.Colorize(sep, format.Dim, useColor))

	for _, s := range sessions {
		marker := "  "
		if reg != nil && s.Account == reg.Main {
			marker = format.Colorize("★ ", format.Yellow, useColor)
		}

		nameStr := format.Colorize(s.Account, format.Cyan, useColor)

		activeTag := ""
		if s.Active {
			activeTag = format.Colorize("[active] ", format.Green, useColor)
		}

		ageStr := formatAge(s.Age)
		topic := sessionTopic(&s)

		fmt.Fprintf(app.Stdout, "%s%-4s  %s%-8s  %s\n",
			marker, nameStr, activeTag, ageStr, topic)
	}
}

// sessionTopic builds a display string showing first and last user messages.
// Format: "first message..." → "last message..." (or just first if same/empty).
func sessionTopic(s *sessionEntry) string {
	first := s.FirstMsg
	last := s.LastMsg

	if first == "" && last == "" {
		if s.Slug != "" {
			return s.Slug
		}
		return s.Project
	}

	if first == "" {
		return "→ " + last
	}
	if last == "" || first == last {
		return first
	}
	return first + "  →  " + last
}

func formatAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
