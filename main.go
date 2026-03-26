package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strings"
	"syscall"

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

// commands maps subcommand names to their definitions.
var commands = map[string]command{
	"setup":      {&setupCmd{}, "One-time interactive setup (accounts + shell wrapper)", catGettingStarted},
	"switch":     {&switchCmd{}, "Switch account (via wrapper: cx switch 5x)", catDailyUse},
	"run":        {&runCmd{}, "Auto-select best account and launch claude", catDailyUse},
	"config":     {&configCmd{}, "Manage accounts, main, metadata", catDailyUse},
	"sessions":   {&sessionsCmd{}, "List recent CC sessions across all accounts", catDailyUse},
	"resume":     {&resumeCmd{}, "Resume a CC session with smart matching", catDailyUse},
	"status":     {&statusCmd{}, "All accounts: auth status + rate limits", catDailyUse},
	"auto":       {&autoCmd{}, "Auto-switching daemon (monitors rate limits)", catMonitoring},
	"dashboard":  {&dashboardCmd{}, "Live TUI dashboard", catMonitoring},
	"insights":   {&insightsCmd{}, "Usage pattern analysis", catMonitoring},
	"predict":    {&predictCmd{}, "Forecast rate limit exhaustion", catMonitoring},
	"web":        {&webCmd{}, "Browser dashboard on localhost", catMonitoring},
	"usage":      {&usageCmd{}, "Usage analysis (daily/weekly/monthly/session/blocks/messages)", catMonitoring},
	"doctor":     {&doctorCmd{}, "Health check all accounts", catMaintenance},
	"sync":       {&syncCmd{}, "Sync config to secondary accounts", catMaintenance},
	"login":      {&loginCmd{}, "Re-authenticate an account", catMaintenance},
	"init":       {&initCmd{}, "Create a new account directory", catMaintenance},
	"watch":      {&watchCmd{}, "Auto-sync daemon (30s interval)", catMaintenance},
	"completion": {&completionCmd{}, "Tab completion (bash/fish/powershell)", catMaintenance},
	"statusline": {&statuslineCmd{}, "CC status bar integration (internal)", catMaintenance},
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
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
	"insights":   "[--all]",
	"auto":       "[--once]",
	"predict":    "[--json]",
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
