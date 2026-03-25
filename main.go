package main

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
)

var version = "dev"

// category groups commands in the help output.
type category int

const (
	catGettingStarted category = iota
	catDailyUse
	catMonitoring
	catMaintenance
)

var categoryNames = map[category]string{
	catGettingStarted: "Getting Started",
	catDailyUse:       "Daily Use",
	catMonitoring:     "Monitoring",
	catMaintenance:    "Maintenance",
}

// categoryOrder defines the display order in help output.
var categoryOrder = []category{
	catGettingStarted,
	catDailyUse,
	catMonitoring,
	catMaintenance,
}

// command represents a CLI subcommand with its handler and metadata.
type command struct {
	fn   func()
	desc string
	cat  category
}

// commands maps subcommand names to their definitions.
var commands = map[string]command{
	"setup":      {runSetup, "One-time interactive setup (accounts + shell wrapper)", catGettingStarted},
	"switch":     {runSwitch, "Switch account (via wrapper: cx switch 5x)", catDailyUse},
	"run":        {runRun, "Auto-select best account and launch claude", catDailyUse},
	"config":     {runConfig, "Manage accounts, primary, metadata", catDailyUse},
	"status":     {runStatus, "All accounts: auth status + rate limits", catDailyUse},
	"dashboard":  {runDashboard, "Live TUI dashboard", catMonitoring},
	"web":        {runWeb, "Browser dashboard on localhost", catMonitoring},
	"usage":      {runUsage, "Usage analysis (daily/weekly/monthly/session/blocks/messages)", catMonitoring},
	"doctor":     {runDoctor, "Health check all accounts", catMaintenance},
	"sync":       {runSync, "Sync config to secondary accounts", catMaintenance},
	"login":      {runLogin, "Re-authenticate an account", catMaintenance},
	"init":       {runInit, "Create a new account directory", catMaintenance},
	"watch":      {runWatch, "Auto-sync daemon (30s interval)", catMaintenance},
	"completion": {runCompletion, "Tab completion (bash/fish/powershell)", catMaintenance},
	"statusline": {runStatusline, "CC status bar integration (internal)", catMaintenance},
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	arg := os.Args[1]

	// Handle aliases for version and help.
	switch arg {
	case "--version", "version":
		printVersion()
		return
	case "--help", "help":
		printHelp()
		return
	}

	cmd, ok := commands[arg]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", arg)
		fmt.Fprintln(os.Stderr, "Run 'cx help' for usage.")
		os.Exit(1)
	}

	cmd.fn()
}

func printVersion() {
	fmt.Printf("cx %s (%s, %s/%s)\n", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// commandUsageHint provides the argument hint shown next to the command name
// in help output. Commands without hints show just the bare name.
var commandUsageHint = map[string]string{
	"config":     "[subcommand]",
	"switch":     "<name>",
	"run":        "[-- args]",
	"web":        "[--port N]",
	"usage":      "<mode>",
	"sync":       "[--force]",
	"login":      "[name]",
	"init":       "<name>",
	"completion": "<shell>",
}

func printHelp() {
	var b strings.Builder

	b.WriteString("cx - Claude Code multi-account manager\n")
	b.WriteString("\nUsage:\n")
	b.WriteString("  cx <command> [options]\n")

	for _, cat := range categoryOrder {
		b.WriteString(fmt.Sprintf("\n%s:\n", categoryNames[cat]))

		// Collect commands in this category, sorted by name.
		var names []string
		for name, cmd := range commands {
			if cmd.cat == cat {
				names = append(names, name)
			}
		}
		sort.Strings(names)

		for _, name := range names {
			cmd := commands[name]
			label := name
			if hint, ok := commandUsageHint[name]; ok {
				label = name + " " + hint
			}
			b.WriteString(fmt.Sprintf("  %-18s %s\n", label, cmd.desc))
		}
	}

	b.WriteString("\n  version            Print version\n")
	b.WriteString("  help               Show this help\n")

	fmt.Print(b.String())
}
