package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/format"
	"github.com/Mike-7777777/cx/internal/platform"
)

const defaultSessionLimit = 10

// sessionEntry holds metadata for one CC session across any account.
type sessionEntry struct {
	ID        string
	Account   string
	ConfigDir string
	Slug      string
	Project   string
	Model     string
	Age       time.Duration
	Tokens    int64
	Active    bool
}

func runSessions() {
	limit := defaultSessionLimit
	accountFilter := ""
	showAll := false

	for i := 2; i < len(os.Args); i++ {
		switch {
		case os.Args[i] == "--all":
			showAll = true
		case os.Args[i] == "--account" && i+1 < len(os.Args):
			accountFilter = os.Args[i+1]
			i++
		case strings.HasPrefix(os.Args[i], "--account="):
			accountFilter = strings.TrimPrefix(os.Args[i], "--account=")
		case os.Args[i] == "--help" || os.Args[i] == "-h":
			fmt.Print(`cx sessions — list recent CC sessions across all accounts

Usage:
  cx sessions [options]

Options:
  --all              Show all sessions (default: last 10)
  --account <name>   Filter by account
`)
			return
		}
	}

	if showAll {
		limit = 0
	}

	sessions := collectSessions(accountFilter, limit)
	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		return
	}

	reg := loadRegistryOrNil()
	useColor := platform.ANSIEnabled()
	printSessionTable(sessions, reg, useColor)
}

// collectSessions gathers session metadata from all accounts.
func collectSessions(accountFilter string, limit int) []sessionEntry {
	reg := loadRegistryOrNil()
	if reg == nil {
		return nil
	}

	// Find active session PIDs.
	activeSessions := make(map[string]bool)
	for _, name := range sortedNames(reg) {
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

	for _, name := range sortedNames(reg) {
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
					ID:        sessionID,
					Account:   name,
					ConfigDir: dir,
					Project:   shortProjectName(proj.Name()),
					Age:       time.Since(info.ModTime()),
					Active:    activeSessions[sessionID],
				}

				// Read first line for model, last lines for slug/tokens.
				readSessionMeta(filepath.Join(projPath, f.Name()), &si)
				all = append(all, si)
			}
		}
	}

	// Sort by most recent first.
	sort.Slice(all, func(i, j int) bool {
		return all[i].Age < all[j].Age
	})

	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}

	return all
}

// readSessionMeta reads the first and last lines of a JSONL file
// to extract model, slug, and token count without parsing the whole file.
func readSessionMeta(path string, si *sessionEntry) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		return
	}

	// First line: get model from first assistant message.
	for _, line := range lines {
		if len(line) < 10 {
			continue
		}
		var entry struct {
			Type    string `json:"type"`
			Message struct {
				Model string `json:"model"`
				Usage struct {
					InputTokens  int64 `json:"input_tokens"`
					OutputTokens int64 `json:"output_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(line), &entry) == nil {
			if entry.Type == "assistant" && entry.Message.Model != "" && si.Model == "" {
				si.Model = shortModelName(entry.Message.Model)
			}
			if entry.Type == "assistant" {
				si.Tokens += entry.Message.Usage.InputTokens + entry.Message.Usage.OutputTokens
			}
		}
	}

	// Scan last lines for slug (from turn_duration system entries).
	for i := len(lines) - 1; i >= 0 && i >= len(lines)-20; i-- {
		var entry struct {
			Type    string `json:"type"`
			Subtype string `json:"subtype"`
			Slug    string `json:"slug"`
		}
		if json.Unmarshal([]byte(lines[i]), &entry) == nil {
			if entry.Type == "system" && entry.Subtype == "turn_duration" && entry.Slug != "" {
				si.Slug = entry.Slug
				break
			}
		}
	}
}

// shortProjectName converts a project slug to a short display name.
// "I--google-drive-homebase" → "homebase"
func shortProjectName(slug string) string {
	parts := strings.Split(slug, "-")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return slug
}

func printSessionTable(sessions []sessionEntry, reg *config.Registry, useColor bool) {
	header := fmt.Sprintf("  %-8s  %-26s  %-8s  %-12s  %-8s  %s",
		"Account", "Session", "Age", "Project", "Model", "Tokens")
	sep := strings.Repeat("━", 82)

	fmt.Println(format.Colorize(header, format.Bold, useColor))
	fmt.Println(format.Colorize(sep, format.Dim, useColor))

	for _, s := range sessions {
		marker := "  "
		if reg != nil && s.Account == reg.Main {
			marker = format.Colorize("★ ", format.Yellow, useColor)
		}

		nameStr := format.Colorize(s.Account, format.Cyan, useColor)

		slug := s.Slug
		if slug == "" {
			slug = s.ID[:12] + "..."
		}
		if s.Active {
			slug = format.Colorize(slug, format.Green+format.Bold, useColor)
		}

		ageStr := formatAge(s.Age)
		tokenStr := formatTokens(s.Tokens)

		fmt.Printf("%s%-8s  %-26s  %-8s  %-12s  %-8s  %s\n",
			marker, nameStr, slug, ageStr, s.Project, s.Model, tokenStr)
	}
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

func formatTokens(t int64) string {
	if t == 0 {
		return "-"
	}
	if t < 1000 {
		return fmt.Sprintf("%d", t)
	}
	return fmt.Sprintf("%.1fk", float64(t)/1000)
}

func sortedNames(reg *config.Registry) []string {
	names := make([]string, 0, len(reg.Accounts))
	for name := range reg.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func loadRegistryOrNil() *config.Registry {
	regPath, err := config.RegistryPath()
	if err != nil {
		return nil
	}
	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		return nil
	}
	return reg
}
