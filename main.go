package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/platform"
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
// Commands migrated to the testable pattern set runner; legacy commands use fn.
type command struct {
	runner Runner // new testable pattern (nil = use fn)
	fn     func() // legacy pattern (migrated incrementally)
	desc   string
	cat    category
}

// commands maps subcommand names to their definitions.
var commands = map[string]command{
	"setup":      {nil, runSetup, "One-time interactive setup (accounts + shell wrapper)", catGettingStarted},
	"switch":     {nil, runSwitch, "Switch account (via wrapper: cx switch 5x)", catDailyUse},
	"run":        {nil, runRun, "Auto-select best account and launch claude", catDailyUse},
	"config":     {nil, runConfig, "Manage accounts, main, metadata", catDailyUse},
	"sessions":   {nil, runSessions, "List recent CC sessions across all accounts", catDailyUse},
	"resume":     {nil, runResume, "Resume a CC session with smart matching", catDailyUse},
	"status":     {nil, runStatus, "All accounts: auth status + rate limits", catDailyUse},
	"dashboard":  {nil, runDashboard, "Live TUI dashboard", catMonitoring},
	"web":        {nil, runWeb, "Browser dashboard on localhost", catMonitoring},
	"usage":      {nil, runUsage, "Usage analysis (daily/weekly/monthly/session/blocks/messages)", catMonitoring},
	"doctor":     {nil, runDoctor, "Health check all accounts", catMaintenance},
	"sync":       {nil, runSync, "Sync config to secondary accounts", catMaintenance},
	"login":      {nil, runLogin, "Re-authenticate an account", catMaintenance},
	"init":       {nil, runInit, "Create a new account directory", catMaintenance},
	"watch":      {nil, runWatch, "Auto-sync daemon (30s interval)", catMaintenance},
	"completion": {nil, runCompletion, "Tab completion (bash/fish/powershell)", catMaintenance},
	"statusline": {nil, runStatusline, "CC status bar integration (internal)", catMaintenance},
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

	if cmd.runner != nil {
		ctx := context.Background()
		app, err := buildApp()
		if err != nil {
			fmt.Fprintf(os.Stderr, "cx: %v\n", err)
			os.Exit(1)
		}
		if err := cmd.runner.Run(ctx, app, os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "cx %s: %v\n", arg, err)
			os.Exit(1)
		}
		return
	}
	cmd.fn()
}

// buildApp constructs a production App with the real registry and OS streams.
func buildApp() (*App, error) {
	regPath, err := config.RegistryPath()
	if err != nil {
		return nil, err
	}
	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		return nil, err
	}
	return &App{
		Registry: reg,
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
		UseColor: platform.ANSIEnabled(),
	}, nil
}

func printVersion() {
	fmt.Printf("cx %s (%s, %s/%s)\n", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// commandUsageHint provides the argument hint shown next to the command name
// in help output. Commands without hints show just the bare name.
var commandUsageHint = map[string]string{
	"config":     "[subcommand]",
	"sessions":   "[--all]",
	"resume":     "[term|--last]",
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
