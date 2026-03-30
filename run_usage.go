package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/usage"
)

// usageCmd implements Runner for the "usage" subcommand.
type usageCmd struct{}

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

// Run performs usage analysis in the requested mode and outputs to app.Stdout.
func (c *usageCmd) Run(_ context.Context, app *App, args []string) error {
	out := app.Stdout
	w := app.Stderr

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

	flagStart := 0

	// First positional arg is mode if it doesn't start with "--".
	if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		switch args[0] {
		case "daily", "session", "blocks", "monthly", "weekly", "messages":
			mode = args[0]
			flagStart = 1
		default:
			return fmt.Errorf("unknown mode %q (expected daily, session, blocks, monthly, weekly, messages)", args[0])
		}
	}

	// Scan remaining args for flags.
	for i := flagStart; i < len(args); i++ {
		switch args[i] {
		case "--json":
			outputFormat = "json"
		case "--format":
			if i+1 >= len(args) {
				return fmt.Errorf("--format requires a value (table, json, csv, md)")
			}
			i++
			switch args[i] {
			case "table", "json", "csv", "md":
				outputFormat = args[i]
			default:
				return fmt.Errorf("unknown format %q (expected table, json, csv, md)", args[i])
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
				return fmt.Errorf("--limit requires a numeric value")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 {
				return fmt.Errorf("--limit must be a positive integer, got %q", args[i])
			}
			limit = n
		case "--account":
			if i+1 >= len(args) {
				return fmt.Errorf("--account requires a value")
			}
			i++
			accountName = args[i]
		case "--all":
			scanAll = true
		case "--all-tools":
			allTools = true
		case "--since":
			if i+1 >= len(args) {
				return fmt.Errorf("--since requires a YYYY-MM-DD value")
			}
			i++
			sinceStr = args[i]
		case "--no-cache":
			noCache = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	// Parse --since into a time boundary.
	var sinceTime time.Time
	if sinceStr != "" {
		t, err := time.Parse("2006-01-02", sinceStr)
		if err != nil {
			return fmt.Errorf("invalid --since date %q: %v", sinceStr, err)
		}
		sinceTime = t
	}

	isJSON := outputFormat == "json"

	// Determine which config directories to scan.
	configDirs, err := resolveConfigDirs(accountName, scanAll)
	if err != nil {
		return err
	}

	// Resolve cache file path (next to the registry, in home dir).
	cachePath := usageCachePath()

	// Daily mode with caching (only when format is table/json and no breakdown/project/compare/subagent needed).
	// --all-tools bypasses the cache because other CLIs are not tracked there.
	if mode == "daily" && !noCache && !allTools && !breakdown && !byProject && !compare && !subagents && outputFormat != "csv" && outputFormat != "md" {
		return doUsageDailyCached(out, w, configDirs, cachePath, sinceTime, outputFormat, showROI, app.UseColor)
	}

	// All other modes require full scan for individual Entry structs.
	var entries []usage.Entry
	if allTools {
		// --all-tools: scan the main config dir plus all other CLI tool dirs.
		mainDir, err := config.DetectConfigDir()
		if err != nil {
			return fmt.Errorf("detecting main config dir: %w", err)
		}
		if err := usage.ScanAllCLIs(mainDir, func(e usage.Entry) {
			entries = append(entries, e)
		}); err != nil {
			fmt.Fprintf(w, "scanning all CLIs: %v\n", err)
		}
	} else {
		for _, dir := range configDirs {
			if err := usage.ScanDir(dir, func(e usage.Entry) {
				entries = append(entries, e)
			}); err != nil {
				fmt.Fprintf(w, "scanning %s: %v\n", dir, err)
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
		fmt.Fprintln(w, "no usage entries found")
		return nil
	}

	// Color is disabled for machine-readable output or when terminal doesn't support it.
	useColor := outputFormat == "table" && !isJSON && app.UseColor

	switch mode {
	case "daily":
		if byProject {
			reports := usage.AggregateByProject(entries)
			if isJSON {
				fprintJSON(out, w, reports)
			} else {
				fmt.Fprint(out, usage.FormatProjectTable(reports, useColor))
			}
		} else if compare {
			pairs := usage.AggregateTrend(allEntries, sinceTime)
			if isJSON {
				fprintJSON(out, w, pairs)
			} else {
				fmt.Fprint(out, usage.FormatTrendTable(pairs, useColor))
			}
		} else {
			reports := usage.AggregateDailies(entries)
			outputDailyReports(out, reports, outputFormat, useColor, breakdown, showROI)
		}

		if subagents {
			bd := usage.AggregateSubagents(entries)
			if isJSON {
				fprintJSON(out, w, bd)
			} else {
				fmt.Fprint(out, usage.FormatSubagentBreakdown(bd, useColor))
			}
		}

		if allTools {
			toolReports := usage.AggregateByCLITool(entries)
			if isJSON {
				fprintJSON(out, w, toolReports)
			} else {
				fmt.Fprint(out, usage.FormatCLIToolTable(toolReports, useColor))
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
			fprintJSON(out, w, entries)
		} else {
			fmt.Fprint(out, usage.FormatMessagesTable(entries, useColor))
		}

	case "monthly":
		reports := usage.AggregateMonthly(entries)
		if isJSON {
			fprintJSON(out, w, reports)
		} else {
			fmt.Fprint(out, usage.FormatMonthlyTable(reports, useColor))
		}
		if showROI {
			totalCost := sumMonthlyReportsCost(reports)
			fmt.Fprint(out, usage.FormatROI(totalCost, totalSubscriptionCost(), useColor))
		}

	case "weekly":
		reports := usage.AggregateWeekly(entries)
		if isJSON {
			fprintJSON(out, w, reports)
		} else {
			fmt.Fprint(out, usage.FormatWeeklyTable(reports, useColor))
		}
		if showROI {
			totalCost := sumWeeklyReportsCost(reports)
			fmt.Fprint(out, usage.FormatROI(totalCost, totalSubscriptionCost(), useColor))
		}

	case "session":
		reports := usage.AggregateSessions(entries)
		if isJSON {
			fprintJSON(out, w, reports)
		} else {
			fmt.Fprint(out, usage.FormatSessionTable(reports, useColor))
		}

	case "blocks":
		reports := usage.AggregateBlocks(entries)
		if isJSON {
			fprintJSON(out, w, reports)
		} else {
			fmt.Fprint(out, usage.FormatBlockTable(reports, useColor))
		}
	}
	return nil
}

// outputDailyReports handles rendering daily reports in all supported formats.
func outputDailyReports(out io.Writer, reports []usage.DailyReport, outputFormat string, useColor, breakdown, showROI bool) {
	switch outputFormat {
	case "json":
		s, _ := usage.FormatJSON(reports)
		fmt.Fprintln(out, s)
	case "csv":
		fmt.Fprint(out, usage.FormatDailyCSV(reports))
	case "md":
		fmt.Fprint(out, usage.FormatDailyMarkdown(reports))
	default: // "table"
		if breakdown {
			fmt.Fprint(out, usage.FormatDailyTableWithBreakdown(reports, useColor))
		} else {
			fmt.Fprint(out, usage.FormatDailyTable(reports, useColor))
		}
	}

	if showROI {
		totalCost := sumDailyReportsCost(reports)
		fmt.Fprint(out, usage.FormatROI(totalCost, totalSubscriptionCost(), useColor))
	}
}

// doUsageDailyCached implements the fully incremental daily report path.
// It loads the cache, scans only changed files, merges new entries into the
// cached daily map, saves the cache, and outputs the report.
func doUsageDailyCached(out, w io.Writer, configDirs []string, cachePath string, sinceTime time.Time, outputFormat string, showROI bool, appUseColor bool) error {
	uc, err := usage.LoadUsageCache(cachePath)
	if err != nil {
		fmt.Fprintf(w, "loading cache: %v\n", err)
	}

	// Scan changed files and merge entries into the daily cache.
	for _, dir := range configDirs {
		if err := usage.ScanDirCached(dir, uc, func(e usage.Entry) {
			dateKey := e.Timestamp.UTC().Format("2006-01-02")
			uc.MergeDailyEntry(dateKey, e)
		}); err != nil {
			fmt.Fprintf(w, "scanning %s: %v\n", dir, err)
		}
	}

	// Save the updated cache.
	if err := uc.Save(); err != nil {
		fmt.Fprintf(w, "saving cache: %v\n", err)
	}

	// Convert cached daily map to reports.
	reports := uc.DailyReports()

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
		fmt.Fprintln(w, "no usage entries found")
		return nil
	}

	isJSON := outputFormat == "json"
	useColor := !isJSON && appUseColor

	if isJSON {
		fprintJSON(out, w, reports)
	} else {
		fmt.Fprint(out, usage.FormatDailyTable(reports, useColor))
	}

	if showROI {
		totalCost := sumDailyReportsCost(reports)
		fmt.Fprint(out, usage.FormatROI(totalCost, totalSubscriptionCost(), useColor))
	}
	return nil
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
		return nil, fmt.Errorf("no accounts in registry; run: cx config add <name>")
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

// fprintJSON marshals the data as indented JSON and writes to out.
func fprintJSON(out, w io.Writer, v any) {
	s, err := usage.FormatJSON(v)
	if err != nil {
		fmt.Fprintf(w, "json marshal: %v\n", err)
		return
	}
	fmt.Fprintln(out, s)
}
