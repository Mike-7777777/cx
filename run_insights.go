package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/usage"
)

// insightsCmd implements Runner for the "insights" subcommand.
type insightsCmd struct{}

// insightsReport is the JSON output structure for the insights command.
type insightsReport struct {
	Hourly     []usage.HourlyReport            `json:"hourly"`
	Models     []usage.ModelDistributionReport `json:"models"`
	Projects   []usage.ProjectReport           `json:"projects"`
	Efficiency usage.EfficiencyMetrics         `json:"efficiency"`
	PeakHours  []usage.HourlyReport            `json:"peak_hours"`
}

const insightsHelp = `cx insights — usage pattern analysis

Usage:
  cx insights [options]

Options:
  --all              Scan all accounts (default: current)
  --since YYYY-MM-DD Filter entries after date
  --json             JSON output
  --help, -h         Show this help

Sections:
  Hourly Distribution    Activity by hour (UTC) with bar chart
  Model Distribution     Cost/token share per model
  Project Breakdown      Cost by project directory
  Efficiency Metrics     Cache hit ratio, avg tokens/msg, I/O ratio
`

// Run performs insights analysis and outputs to app.Stdout.
func (c *insightsCmd) Run(_ context.Context, app *App, args []string) error {
	out := app.Stdout
	w := app.Stderr

	flags, _ := parseFlags(args, "json", "all", "since", "dir", "help", "h")

	if _, ok := flags["help"]; ok {
		fmt.Fprint(out, insightsHelp)
		return nil
	}
	if _, ok := flags["h"]; ok {
		fmt.Fprint(out, insightsHelp)
		return nil
	}

	_, isJSON := flags["json"]
	_, scanAll := flags["all"]
	sinceStr := flags["since"]
	dirOverride := flags["dir"]

	// Parse --since into a time boundary.
	var sinceTime time.Time
	if sinceStr != "" {
		t, err := time.Parse("2006-01-02", sinceStr)
		if err != nil {
			return fmt.Errorf("invalid --since date %q: %v", sinceStr, err)
		}
		sinceTime = t
	}

	// Resolve which config directories to scan.
	var configDirs []string
	if dirOverride != "" {
		configDirs = []string{dirOverride}
	} else if scanAll {
		dirs, err := allRegistryDirs()
		if err != nil {
			return err
		}
		configDirs = dirs
	} else {
		dir, err := config.DetectConfigDir()
		if err != nil {
			return fmt.Errorf("detecting config dir: %w", err)
		}
		configDirs = []string{dir}
	}

	// Collect all entries.
	var entries []usage.Entry
	for _, dir := range configDirs {
		if err := usage.ScanDir(dir, func(e usage.Entry) {
			entries = append(entries, e)
		}); err != nil {
			fmt.Fprintf(w, "scanning %s: %v\n", dir, err)
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
		fmt.Fprintln(w, "no usage entries found")
		return nil
	}

	useColor := !isJSON && app.UseColor

	if isJSON {
		report := insightsReport{
			Hourly:     usage.AggregateHourly(entries),
			Models:     usage.AggregateModelDistribution(entries),
			Projects:   usage.AggregateByProject(entries),
			Efficiency: usage.CalculateEfficiency(entries),
			PeakHours:  usage.FindPeakHours(entries, 3),
		}
		fprintJSON(out, w, report)
		return nil
	}

	fmt.Fprint(out, usage.FormatHourlyTable(usage.AggregateHourly(entries), useColor))
	fmt.Fprint(out, usage.FormatModelDistributionTable(usage.AggregateModelDistribution(entries), useColor))
	fmt.Fprint(out, "\n"+usage.FormatProjectTable(usage.AggregateByProject(entries), useColor))
	fmt.Fprint(out, usage.FormatEfficiency(usage.CalculateEfficiency(entries), useColor))

	return nil
}
