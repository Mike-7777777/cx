package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/MasaYan24/cc-monitor/internal/config"
	"github.com/MasaYan24/cc-monitor/internal/usage"
)

func runUsage() {
	// Parse subcommand: cc-monitor usage [daily|session|blocks] [flags]
	mode := "daily"
	var jsonOutput bool
	var accountName string
	var scanAll bool
	var sinceStr string

	args := os.Args[2:]
	flagStart := 0

	// First positional arg is mode if it doesn't start with "--".
	if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		switch args[0] {
		case "daily", "session", "blocks":
			mode = args[0]
			flagStart = 1
		default:
			fmt.Fprintf(os.Stderr, "cc-monitor usage: unknown mode %q (expected daily, session, blocks)\n", args[0])
			os.Exit(1)
		}
	}

	// Scan remaining args for flags.
	for i := flagStart; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--account":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "cc-monitor usage: --account requires a value")
				os.Exit(1)
			}
			i++
			accountName = args[i]
		case "--all":
			scanAll = true
		case "--since":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "cc-monitor usage: --since requires a YYYY-MM-DD value")
				os.Exit(1)
			}
			i++
			sinceStr = args[i]
		default:
			fmt.Fprintf(os.Stderr, "cc-monitor usage: unknown flag %q\n", args[i])
			os.Exit(1)
		}
	}

	// Parse --since into a time boundary.
	var sinceTime time.Time
	if sinceStr != "" {
		t, err := time.Parse("2006-01-02", sinceStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cc-monitor usage: invalid --since date %q: %v\n", sinceStr, err)
			os.Exit(1)
		}
		sinceTime = t
	}

	// Determine which config directories to scan.
	configDirs, err := resolveConfigDirs(accountName, scanAll)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cc-monitor usage: %v\n", err)
		os.Exit(1)
	}

	// Collect entries from all config dirs.
	var entries []usage.Entry
	for _, dir := range configDirs {
		if err := usage.ScanDir(dir, func(e usage.Entry) {
			entries = append(entries, e)
		}); err != nil {
			fmt.Fprintf(os.Stderr, "cc-monitor usage: scanning %s: %v\n", dir, err)
		}
	}

	// Apply --since filter.
	if !sinceTime.IsZero() {
		filtered := entries[:0]
		for _, e := range entries {
			if !e.Timestamp.Before(sinceTime) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "cc-monitor usage: no usage entries found")
		os.Exit(0)
	}

	// Aggregate and format.
	switch mode {
	case "daily":
		reports := usage.AggregateDailies(entries)
		if jsonOutput {
			printJSON(reports)
		} else {
			fmt.Print(usage.FormatDailyTable(reports))
		}
	case "session":
		reports := usage.AggregateSessions(entries)
		if jsonOutput {
			printJSON(reports)
		} else {
			fmt.Print(usage.FormatSessionTable(reports))
		}
	case "blocks":
		reports := usage.AggregateBlocks(entries)
		if jsonOutput {
			printJSON(reports)
		} else {
			fmt.Print(usage.FormatBlockTable(reports))
		}
	}
}

// resolveConfigDirs returns the list of config directories to scan based on flags.
func resolveConfigDirs(accountName string, scanAll bool) ([]string, error) {
	if accountName != "" && scanAll {
		return nil, fmt.Errorf("--account and --all are mutually exclusive")
	}

	if scanAll {
		return allRegistryDirs()
	}

	if accountName != "" {
		return registryAccountDir(accountName)
	}

	// Default: current account from environment or detection.
	dir, err := config.DetectConfigDir()
	if err != nil {
		return nil, fmt.Errorf("detecting config dir: %w", err)
	}
	return []string{dir}, nil
}

// allRegistryDirs returns config directories for every registered account.
func allRegistryDirs() ([]string, error) {
	regPath, err := config.RegistryPath()
	if err != nil {
		return nil, err
	}
	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		return nil, err
	}
	if len(reg.Accounts) == 0 {
		return nil, fmt.Errorf("no accounts in registry; run: cc-monitor init <name>")
	}

	dirs := make([]string, 0, len(reg.Accounts))
	for name := range reg.Accounts {
		dir, err := reg.ResolveConfigDir(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cc-monitor usage: skipping account %q: %v\n", name, err)
			continue
		}
		dirs = append(dirs, dir)
	}
	return dirs, nil
}

// registryAccountDir returns the config directory for a specific named account.
func registryAccountDir(name string) ([]string, error) {
	regPath, err := config.RegistryPath()
	if err != nil {
		return nil, err
	}
	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		return nil, err
	}
	dir, err := reg.ResolveConfigDir(name)
	if err != nil {
		return nil, err
	}
	return []string{dir}, nil
}

// printJSON marshals the data as indented JSON and prints to stdout.
func printJSON(v any) {
	out, err := usage.FormatJSON(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cc-monitor usage: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(out)
}
