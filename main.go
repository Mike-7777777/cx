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
type command struct {
	runner Runner
	desc   string
	cat    category
}

// legacyCmd wraps a legacy func() as a Runner. This is a migration aid —
// each command will be properly migrated to accept (ctx, app, args) and
// this wrapper will be removed.
type legacyCmd struct {
	fn func()
}

func (c *legacyCmd) Run(_ context.Context, _ *App, _ []string) error {
	c.fn()
	return nil
}

func legacy(fn func()) Runner {
	return &legacyCmd{fn: fn}
}

// commands maps subcommand names to their definitions.
var commands = map[string]command{
	"setup":      {legacy(runSetup), "One-time interactive setup (accounts + shell wrapper)", catGettingStarted},
	"switch":     {&switchCmd{}, "Switch account (via wrapper: cx switch 5x)", catDailyUse},
	"run":        {legacy(runRun), "Auto-select best account and launch claude", catDailyUse},
	"config":     {legacy(runConfig), "Manage accounts, main, metadata", catDailyUse},
	"sessions":   {legacy(runSessions), "List recent CC sessions across all accounts", catDailyUse},
	"resume":     {legacy(runResume), "Resume a CC session with smart matching", catDailyUse},
	"status":     {&statusCmd{}, "All accounts: auth status + rate limits", catDailyUse},
	"dashboard":  {legacy(runDashboard), "Live TUI dashboard", catMonitoring},
	"web":        {legacy(runWeb), "Browser dashboard on localhost", catMonitoring},
	"usage":      {legacy(runUsage), "Usage analysis (daily/weekly/monthly/session/blocks/messages)", catMonitoring},
	"doctor":     {legacy(runDoctor), "Health check all accounts", catMaintenance},
	"sync":       {legacy(runSync), "Sync config to secondary accounts", catMaintenance},
	"login":      {&loginCmd{}, "Re-authenticate an account", catMaintenance},
	"init":       {&initCmd{}, "Create a new account directory", catMaintenance},
	"watch":      {&watchCmd{}, "Auto-sync daemon (30s interval)", catMaintenance},
	"completion": {&completionCmd{}, "Tab completion (bash/fish/powershell)", catMaintenance},
	"statusline": {legacy(runStatusline), "CC status bar integration (internal)", catMaintenance},
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
