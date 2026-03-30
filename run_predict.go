package main

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/Mike-7777777/cx/internal/format"
	"github.com/Mike-7777777/cx/internal/usage"
)

// predictCmd implements Runner for the "predict" subcommand.
type predictCmd struct{}

// predictRow holds all data for one account's forecast row.
type predictRow struct {
	name      string
	fivePct   float64
	fiveReset time.Duration
	estimate  usage.ExhaustionEstimate
	velocity  usage.Velocity
}

const predictHelp = `cx predict — forecast rate limit exhaustion

Usage:
  cx predict [options]

Options:
  --json        JSON output
  --help, -h    Show this help

Output columns:
  Account       Account name
  5h %          Current 5-hour window usage
  Reset In      Time until window resets
  Velocity      Messages per hour (last 5h)
  Exhausts In   Estimated time to hit rate limit
  Status        SAFE / OK / WARNING / LIMIT

Status legend:
  SAFE     Will not exhaust before window resets
  OK       Exhausts >30m from now
  WARNING  Exhausts within 30m
  LIMIT    Already at rate limit
`

// Run collects rate limit data and velocity for all accounts, then prints a forecast table.
func (c *predictCmd) Run(_ context.Context, app *App, args []string) error {
	out := app.Stdout
	w := app.Stderr

	flags, _ := parseFlags(args, "json", "help", "h")

	if _, ok := flags["help"]; ok {
		fmt.Fprint(out, predictHelp)
		return nil
	}
	if _, ok := flags["h"]; ok {
		fmt.Fprint(out, predictHelp)
		return nil
	}

	_, isJSON := flags["json"]
	reg := app.Registry

	if len(reg.Accounts) == 0 {
		fmt.Fprintln(out, "No accounts configured. Run: cx config add <name>")
		return nil
	}

	now := time.Now()

	// Collect names in stable order.
	names := make([]string, 0, len(reg.Accounts))
	for name := range reg.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)

	rows := make([]predictRow, 0, len(names))

	// Load usage cache once outside the loop.
	cachePath := usageCachePath()
	uc, _ := usage.LoadUsageCache(cachePath)

	for _, name := range names {
		dir, err := reg.ResolveConfigDir(name)
		if err != nil {
			continue
		}

		fivePct, fiveReset, _, _ := fiveHourStats(dir)
		est := usage.EstimateExhaustion(fivePct, fiveReset, now)

		var entries []usage.Entry
		_ = usage.ScanDirCached(dir, uc, func(e usage.Entry) {
			entries = append(entries, e)
		})
		vel := usage.CalculateVelocity(entries, 5*time.Hour)

		rows = append(rows, predictRow{
			name:      name,
			fivePct:   fivePct,
			fiveReset: fiveReset,
			estimate:  est,
			velocity:  vel,
		})
	}

	_ = uc.Save()

	if isJSON {
		type jsonRow struct {
			Account  string                   `json:"account"`
			FivePct  float64                  `json:"five_hour_pct"`
			Estimate usage.ExhaustionEstimate `json:"estimate"`
			Velocity usage.Velocity           `json:"velocity"`
		}
		out2 := make([]jsonRow, 0, len(rows))
		for _, r := range rows {
			out2 = append(out2, jsonRow{
				Account:  r.name,
				FivePct:  r.fivePct,
				Estimate: r.estimate,
				Velocity: r.velocity,
			})
		}
		fprintJSON(out, w, out2)
		return nil
	}

	printPredictTable(out, rows, app.UseColor)
	printPredictAdvice(out, rows, app.UseColor)
	return nil
}

func printPredictTable(w io.Writer, rows []predictRow, useColor bool) {
	header := fmt.Sprintf("  %-12s  %-6s  %-10s  %-12s  %-12s  %s",
		"Account", "5h %", "Reset In", "Velocity", "Exhausts In", "Status")
	sep := "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

	fmt.Fprintln(w, format.Colorize(header, format.Bold, useColor))
	fmt.Fprintln(w, format.Colorize(sep, format.Dim, useColor))

	for _, row := range rows {
		nameStr := format.Colorize(fmt.Sprintf("%-12s", row.name), format.Cyan, useColor)
		pctStr := fmt.Sprintf("%3.0f%%", row.fivePct)

		resetStr := "-"
		if row.fiveReset > 0 {
			resetStr = format.FormatDuration(row.fiveReset)
		}

		velStr := fmt.Sprintf("%.1f msg/h", row.velocity.MsgsPerHour)

		exhaustStr := "never"
		if row.estimate.Exhausted {
			exhaustStr = "now"
		} else if row.estimate.TimeToExhaust > 0 {
			exhaustStr = format.FormatDuration(row.estimate.TimeToExhaust)
		}

		status, statusColor := predictStatus(row.estimate)
		statusStr := format.Colorize(status, statusColor, useColor)

		fmt.Fprintf(w, "  %s  %-6s  %-10s  %-12s  %-12s  %s\n",
			nameStr, pctStr, resetStr, velStr, exhaustStr, statusStr)
	}
}

// predictStatus returns the status label and color for an account.
func predictStatus(est usage.ExhaustionEstimate) (string, string) {
	switch {
	case est.Exhausted:
		return "LIMIT", format.Red
	case est.TimeToExhaust > 0 && est.TimeToExhaust < 30*time.Minute:
		return "WARNING", format.Yellow
	case est.TimeToExhaust > 0:
		return "OK", format.Green
	default:
		return "SAFE", format.Green
	}
}

func printPredictAdvice(w io.Writer, rows []predictRow, useColor bool) {
	fmt.Fprintln(w)

	var atRisk []predictRow
	for _, r := range rows {
		if r.estimate.Exhausted || (r.estimate.TimeToExhaust > 0 && r.estimate.TimeToExhaust < time.Hour) {
			atRisk = append(atRisk, r)
		}
	}

	if len(atRisk) == 0 {
		fmt.Fprintln(w, format.Colorize("All accounts healthy. No action needed.", format.Green, useColor))
		return
	}

	for _, r := range atRisk {
		if r.estimate.Exhausted {
			msg := fmt.Sprintf("! %s is at rate limit. Switch to another account.", r.name)
			fmt.Fprintln(w, format.Colorize(msg, format.Red, useColor))
		} else {
			msg := fmt.Sprintf("! %s will exhaust in %s at current pace. Consider switching.",
				r.name, format.FormatDuration(r.estimate.TimeToExhaust))
			fmt.Fprintln(w, format.Colorize(msg, format.Yellow, useColor))
		}
	}
}
