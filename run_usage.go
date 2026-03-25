package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/platform"
	"github.com/Mike-7777777/cx/internal/usage"
)

// usageCacheFilename is the name of the incremental cache file stored in the
// user's config directory (next to the registry).
const usageCacheFilename = "cx-usage-cache.json"

// subscriptionCosts maps plan names to monthly costs in USD.
var subscriptionCosts = map[string]float64{
	"Max 20x": 200.0,
	"Max 5x":  100.0,
}

// totalSubscriptionCost returns the combined monthly subscription cost.
func totalSubscriptionCost() float64 {
	var total float64
	for _, cost := range subscriptionCosts {
		total += cost
	}
	return total
}

func runUsage() {
	usage.CheckPricingStaleness()

	// Parse subcommand: cx usage [daily|session|blocks|monthly|weekly|messages] [flags]
	mode := "daily"
	var accountName string
	var scanAll bool
	var allTools bool
	var sinceStr string
	var noCache bool
	var breakdown bool
	var showROI bool
	var byProject bool
	var compare bool
	var subagents bool
	var limit int
	outputFormat := "table" // table, json, csv, md

	args := os.Args[2:]
	flagStart := 0

	// First positional arg is mode if it doesn't start with "--".
	if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		switch args[0] {
		case "daily", "session", "blocks", "monthly", "weekly", "messages":
			mode = args[0]
			flagStart = 1
		default:
			fmt.Fprintf(os.Stderr, "cx usage: unknown mode %q (expected daily, session, blocks, monthly, weekly, messages)\n", args[0])
			os.Exit(1)
		}
	}

	// Scan remaining args for flags.
	for i := flagStart; i < len(args); i++ {
		switch args[i] {
		case "--json":
			outputFormat = "json"
		case "--format":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "cx usage: --format requires a value (table, json, csv, md)")
				os.Exit(1)
			}
			i++
			switch args[i] {
			case "table", "json", "csv", "md":
				outputFormat = args[i]
			default:
				fmt.Fprintf(os.Stderr, "cx usage: unknown format %q (expected table, json, csv, md)\n", args[i])
				os.Exit(1)
			}
		case "--breakdown":
			breakdown = true
		case "--roi":
			showROI = true
		case "--by-project":
			byProject = true
		case "--compare":
			compare = true
		case "--subagents":
			subagents = true
		case "--limit":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "cx usage: --limit requires a numeric value")
				os.Exit(1)
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 {
				fmt.Fprintf(os.Stderr, "cx usage: --limit must be a positive integer, got %q\n", args[i])
				os.Exit(1)
			}
			limit = n
		case "--account":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "cx usage: --account requires a value")
				os.Exit(1)
			}
			i++
			accountName = args[i]
		case "--all":
			scanAll = true
		case "--all-tools":
			allTools = true
		case "--since":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "cx usage: --since requires a YYYY-MM-DD value")
				os.Exit(1)
			}
			i++
			sinceStr = args[i]
		case "--no-cache":
			noCache = true
		default:
			fmt.Fprintf(os.Stderr, "cx usage: unknown flag %q\n", args[i])
			os.Exit(1)
		}
	}

	// Parse --since into a time boundary.
	var sinceTime time.Time
	if sinceStr != "" {
		t, err := time.Parse("2006-01-02", sinceStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cx usage: invalid --since date %q: %v\n", sinceStr, err)
			os.Exit(1)
		}
		sinceTime = t
	}

	isJSON := outputFormat == "json"

	// Determine which config directories to scan.
	configDirs, err := resolveConfigDirs(accountName, scanAll)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cx usage: %v\n", err)
		os.Exit(1)
	}

	// Resolve cache file path (next to the registry, in home dir).
	cachePath := usageCachePath()

	// Daily mode with caching (only when format is table/json and no breakdown/project/compare/subagent needed).
	// --all-tools bypasses the cache because other CLIs are not tracked there.
	if mode == "daily" && !noCache && !allTools && !breakdown && !byProject && !compare && !subagents && outputFormat != "csv" && outputFormat != "md" {
		runUsageDailyCached(configDirs, cachePath, sinceTime, outputFormat, showROI)
		return
	}

	// All other modes require full scan for individual Entry structs.
	var entries []usage.Entry
	if allTools {
		// --all-tools: scan the main config dir plus all other CLI tool dirs.
		mainDir, err := config.DetectConfigDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "cx usage: detecting main config dir: %v\n", err)
			os.Exit(1)
		}
		if err := usage.ScanAllCLIs(mainDir, func(e usage.Entry) {
			entries = append(entries, e)
		}); err != nil {
			fmt.Fprintf(os.Stderr, "cx usage: scanning all CLIs: %v\n", err)
		}
	} else {
		for _, dir := range configDirs {
			if err := usage.ScanDir(dir, func(e usage.Entry) {
				entries = append(entries, e)
			}); err != nil {
				fmt.Fprintf(os.Stderr, "cx usage: scanning %s: %v\n", dir, err)
			}
		}
	}

	// Keep unfiltered entries for --compare mode (AggregateTrend needs previous period data).
	allEntries := entries

	// Apply --since filter (skipped for compare mode which handles its own time ranges).
	if !sinceTime.IsZero() && !compare {
		filtered := entries[:0]
		for _, e := range entries {
			if !e.Timestamp.Before(sinceTime) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	if len(entries) == 0 && !compare {
		fmt.Fprintln(os.Stderr, "cx usage: no usage entries found")
		os.Exit(0)
	}

	// Color is disabled for machine-readable output or when terminal doesn't support it.
	useColor := outputFormat == "table" && !isJSON && platform.ANSIEnabled()

	switch mode {
	case "daily":
		if byProject {
			reports := usage.AggregateByProject(entries)
			if isJSON {
				printJSON(reports)
			} else {
				fmt.Print(usage.FormatProjectTable(reports, useColor))
			}
		} else if compare {
			pairs := usage.AggregateTrend(allEntries, sinceTime)
			if isJSON {
				printJSON(pairs)
			} else {
				fmt.Print(usage.FormatTrendTable(pairs, useColor))
			}
		} else {
			reports := usage.AggregateDailies(entries)
			outputDailyReports(reports, outputFormat, useColor, breakdown, showROI)
		}

		if subagents {
			bd := usage.AggregateSubagents(entries)
			if isJSON {
				printJSON(bd)
			} else {
				fmt.Print(usage.FormatSubagentBreakdown(bd, useColor))
			}
		}

		if allTools {
			toolReports := usage.AggregateByCLITool(entries)
			if isJSON {
				printJSON(toolReports)
			} else {
				fmt.Print(usage.FormatCLIToolTable(toolReports, useColor))
			}
		}

	case "messages":
		// Sort by timestamp descending (newest first).
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Timestamp.After(entries[j].Timestamp)
		})
		// Apply limit (default 50).
		msgLimit := limit
		if msgLimit == 0 {
			msgLimit = 50
		}
		if msgLimit > len(entries) {
			msgLimit = len(entries)
		}
		entries = entries[:msgLimit]

		if isJSON {
			printJSON(entries)
		} else {
			fmt.Print(usage.FormatMessagesTable(entries, useColor))
		}

	case "monthly":
		reports := usage.AggregateMonthly(entries)
		if isJSON {
			printJSON(reports)
		} else {
			fmt.Print(usage.FormatMonthlyTable(reports, useColor))
		}
		if showROI {
			totalCost := sumMonthlyReportsCost(reports)
			fmt.Print(usage.FormatROI(totalCost, totalSubscriptionCost(), useColor))
		}

	case "weekly":
		reports := usage.AggregateWeekly(entries)
		if isJSON {
			printJSON(reports)
		} else {
			fmt.Print(usage.FormatWeeklyTable(reports, useColor))
		}
		if showROI {
			totalCost := sumWeeklyReportsCost(reports)
			fmt.Print(usage.FormatROI(totalCost, totalSubscriptionCost(), useColor))
		}

	case "session":
		reports := usage.AggregateSessions(entries)
		if isJSON {
			printJSON(reports)
		} else {
			fmt.Print(usage.FormatSessionTable(reports, useColor))
		}

	case "blocks":
		reports := usage.AggregateBlocks(entries)
		if isJSON {
			printJSON(reports)
		} else {
			fmt.Print(usage.FormatBlockTable(reports, useColor))
		}
	}
}

// outputDailyReports handles rendering daily reports in all supported formats.
func outputDailyReports(reports []usage.DailyReport, outputFormat string, useColor, breakdown, showROI bool) {
	switch outputFormat {
	case "json":
		printJSON(reports)
	case "csv":
		fmt.Print(usage.FormatDailyCSV(reports))
	case "md":
		fmt.Print(usage.FormatDailyMarkdown(reports))
	default: // "table"
		if breakdown {
			fmt.Print(usage.FormatDailyTableWithBreakdown(reports, useColor))
		} else {
			fmt.Print(usage.FormatDailyTable(reports, useColor))
		}
	}

	if showROI {
		totalCost := sumDailyReportsCost(reports)
		fmt.Print(usage.FormatROI(totalCost, totalSubscriptionCost(), useColor))
	}
}

// runUsageDailyCached implements the fully incremental daily report path.
// It loads the cache, scans only changed files, merges new entries into the
// cached daily map, saves the cache, and outputs the report.
func runUsageDailyCached(configDirs []string, cachePath string, sinceTime time.Time, outputFormat string, showROI bool) {
	cache, err := usage.LoadUsageCache(cachePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cx usage: loading cache: %v\n", err)
	}

	// Scan changed files and merge entries into the daily cache.
	for _, dir := range configDirs {
		if err := usage.ScanDirCached(dir, cache, func(e usage.Entry) {
			dateKey := e.Timestamp.UTC().Format("2006-01-02")
			cache.MergeDailyEntry(dateKey, e)
		}); err != nil {
			fmt.Fprintf(os.Stderr, "cx usage: scanning %s: %v\n", dir, err)
		}
	}

	// Save the updated cache.
	if err := cache.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "cx usage: saving cache: %v\n", err)
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
		fmt.Fprintln(os.Stderr, "cx usage: no usage entries found")
		os.Exit(0)
	}

	isJSON := outputFormat == "json"
	useColor := !isJSON && platform.ANSIEnabled()

	if isJSON {
		printJSON(reports)
	} else {
		fmt.Print(usage.FormatDailyTable(reports, useColor))
	}

	if showROI {
		totalCost := sumDailyReportsCost(reports)
		fmt.Print(usage.FormatROI(totalCost, totalSubscriptionCost(), useColor))
	}
}

// sumDailyReportsCost sums up the total API cost across all daily reports.
func sumDailyReportsCost(reports []usage.DailyReport) float64 {
	var total float64
	for _, r := range reports {
		total += r.Summary.CostUSD
	}
	return total
}

// sumMonthlyReportsCost sums up the total API cost across all monthly reports.
func sumMonthlyReportsCost(reports []usage.MonthlyReport) float64 {
	var total float64
	for _, r := range reports {
		total += r.Summary.CostUSD
	}
	return total
}

// sumWeeklyReportsCost sums up the total API cost across all weekly reports.
func sumWeeklyReportsCost(reports []usage.WeeklyReport) float64 {
	var total float64
	for _, r := range reports {
		total += r.Summary.CostUSD
	}
	return total
}

// usageCachePath returns the path for the usage cache file.
func usageCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), usageCacheFilename)
	}
	return filepath.Join(home, ".config", "cx", usageCacheFilename)
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
		return nil, fmt.Errorf("no accounts in registry; run: cx init <name>")
	}

	dirs := make([]string, 0, len(reg.Accounts))
	for name := range reg.Accounts {
		dir, err := reg.ResolveConfigDir(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cx usage: skipping account %q: %v\n", name, err)
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
		fmt.Fprintf(os.Stderr, "cx usage: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(out)
}
