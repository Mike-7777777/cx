package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Mike-7777777/cc-monitor/internal/config"
	"github.com/Mike-7777777/cc-monitor/internal/platform"
	"github.com/Mike-7777777/cc-monitor/internal/usage"
)

// usageCacheFilename is the name of the incremental cache file stored in the
// user's config directory (next to the registry).
const usageCacheFilename = "cc-monitor-usage-cache.json"

func runUsage() {
	// Parse subcommand: cc-monitor usage [daily|session|blocks] [flags]
	mode := "daily"
	var jsonOutput bool
	var accountName string
	var scanAll bool
	var sinceStr string
	var noCache bool

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
		case "--no-cache":
			noCache = true
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

	// Resolve cache file path (next to the registry, in home dir).
	cachePath := usageCachePath()

	// Daily mode with caching: fully incremental via cached daily map.
	if mode == "daily" && !noCache {
		runUsageDailyCached(configDirs, cachePath, sinceTime, jsonOutput)
		return
	}

	// Session/block modes (or --no-cache daily): full scan required because
	// we need individual Entry structs with session IDs and timestamps.
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

	// Color is disabled when --json is set (machine output) or terminal doesn't support it.
	useColor := !jsonOutput && platform.ANSIEnabled()

	switch mode {
	case "daily":
		// Only reachable with --no-cache.
		reports := usage.AggregateDailies(entries)
		if jsonOutput {
			printJSON(reports)
		} else {
			fmt.Print(usage.FormatDailyTable(reports, useColor))
		}
	case "session":
		reports := usage.AggregateSessions(entries)
		if jsonOutput {
			printJSON(reports)
		} else {
			fmt.Print(usage.FormatSessionTable(reports, useColor))
		}
	case "blocks":
		reports := usage.AggregateBlocks(entries)
		if jsonOutput {
			printJSON(reports)
		} else {
			fmt.Print(usage.FormatBlockTable(reports, useColor))
		}
	}
}

// runUsageDailyCached implements the fully incremental daily report path.
// It loads the cache, scans only changed files, merges new entries into the
// cached daily map, saves the cache, and outputs the report.
func runUsageDailyCached(configDirs []string, cachePath string, sinceTime time.Time, jsonOutput bool) {
	cache, err := usage.LoadUsageCache(cachePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cc-monitor usage: loading cache: %v\n", err)
	}

	// Scan changed files and merge entries into the daily cache.
	for _, dir := range configDirs {
		if err := usage.ScanDirCached(dir, cache, func(e usage.Entry) {
			dateKey := e.Timestamp.UTC().Format("2006-01-02")
			cache.MergeDailyEntry(dateKey, e)
		}); err != nil {
			fmt.Fprintf(os.Stderr, "cc-monitor usage: scanning %s: %v\n", dir, err)
		}
	}

	// Save the updated cache.
	if err := cache.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "cc-monitor usage: saving cache: %v\n", err)
	}

	// Convert cached daily map to reports.
	reports := cache.DailyReports()

	// Apply --since filter.
	if !sinceTime.IsZero() {
		sinceKey := sinceTime.Format("2006-01-02")
		filtered := reports[:0]
		for _, r := range reports {
			if r.Date >= sinceKey {
				filtered = append(filtered, r)
			}
		}
		reports = filtered
	}

	// Sort by date ascending.
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Date < reports[j].Date
	})

	if len(reports) == 0 {
		fmt.Fprintln(os.Stderr, "cc-monitor usage: no usage entries found")
		os.Exit(0)
	}

	useColor := !jsonOutput && platform.ANSIEnabled()

	if jsonOutput {
		printJSON(reports)
	} else {
		fmt.Print(usage.FormatDailyTable(reports, useColor))
	}
}

// usageCachePath returns the path for the usage cache file.
func usageCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), usageCacheFilename)
	}
	return filepath.Join(home, ".config", "cc-monitor", usageCacheFilename)
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
