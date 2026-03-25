package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Mike-7777777/cx/internal/cache"
	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/format"
	"github.com/Mike-7777777/cx/internal/platform"
)

const (
	staleThreshold    = 10 * time.Minute
	allStaleThreshold = 1 * time.Hour
	noDataMarker      = "no data"
	stalePrefix       = "stale"
)

type statusRow struct {
	name          string
	fivePct       float64
	fiveResetStr  string
	sevenPct      float64
	sevenResetStr string
	note          string
	hasData       bool
	isStale       bool
	authOK        bool
	isMain        bool
	tier          string
	email         string
}

func runStatus() {
	regPath, err := config.RegistryPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cx status: %v\n", err)
		os.Exit(1)
	}

	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cx status: %v\n", err)
		os.Exit(1)
	}

	if len(reg.Accounts) == 0 {
		fmt.Println("No accounts configured. Run: cx init <name>")
		return
	}

	// Collect account names in stable order.
	names := make([]string, 0, len(reg.Accounts))
	for name := range reg.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)

	rows := make([]statusRow, 0, len(names))
	allStale := true

	for _, name := range names {
		dir, err := reg.ResolveConfigDir(name)
		if err != nil {
			rows = append(rows, statusRow{name: name, note: noDataMarker})
			continue
		}

		rc, err := cache.ReadRateCache(filepath.Join(dir, "rate-cache.json"))
		row := buildRow(name, rc, err)
		row.authOK = checkCredentials(dir) == credentialOK
		row.isMain = name == reg.Main
		info := readAccountInfo(dir)
		row.tier = info.Tier
		if info.Email != "" {
			row.email = info.Email
		} else if acc, ok := reg.Accounts[name]; ok && acc.Email != "" {
			row.email = acc.Email
		}
		rows = append(rows, row)

		if row.hasData && !row.isStale {
			allStale = false
		}
	}

	useColor := platform.ANSIEnabled()
	printTable(rows, useColor)
	printRecommendation(rows, allStale, useColor)
}

func buildRow(name string, rc *cache.RateCache, readErr error) statusRow {
	row := statusRow{name: name}

	if readErr != nil || rc == nil || rc.RateLimits == nil {
		row.note = noDataMarker
		return row
	}

	row.hasData = true
	age := rc.Age()

	if age > staleThreshold {
		row.isStale = true
		mins := int(age.Minutes())
		row.note = fmt.Sprintf("%s %dm", stalePrefix, mins)
	}

	rl := rc.RateLimits

	if rl.FiveHour != nil {
		if rl.FiveHour.IsReset() {
			row.fivePct = 0
			row.fiveResetStr = "reset"
		} else {
			row.fivePct = rl.FiveHour.UsedPercentage
			row.fiveResetStr = format.FormatDuration(rl.FiveHour.TimeToReset())
		}
	} else {
		row.note = noDataMarker
		row.hasData = false
		return row
	}

	if rl.SevenDay != nil {
		row.sevenPct = rl.SevenDay.UsedPercentage
		if rl.SevenDay.IsReset() {
			row.sevenResetStr = "reset"
		} else {
			// Show the actual reset date/time (e.g., "Tue 08:00").
			resetTime := time.Unix(rl.SevenDay.ResetsAt, 0).Local()
			row.sevenResetStr = resetTime.Format("Mon 15:04")
		}
	}

	return row
}

func printTable(rows []statusRow, useColor bool) {
	header := fmt.Sprintf("  %-10s  %-8s  %-6s  %-14s  %-10s  %-14s  %-10s  %s",
		"Account", "Tier", "Auth", "5h Usage", "5h Reset", "7d Usage", "7d Reset", "Note")
	sep := "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

	fmt.Println(format.Colorize(header, format.Bold, useColor))
	fmt.Println(format.Colorize(sep, format.Dim, useColor))

	for _, row := range rows {
		marker := "  "
		if row.isMain {
			marker = format.Colorize("★ ", format.Yellow, useColor)
		}

		nameStr := format.Colorize(row.name, format.Cyan, useColor)
		tierStr := format.Colorize(fmt.Sprintf("%-8s", row.tier), format.Dim, useColor)

		authStr := format.Colorize("OK", format.Green, useColor)
		if !row.authOK {
			authStr = format.Colorize("FAIL", format.Red, useColor)
		}

		if !row.hasData {
			noteStr := format.Colorize(noDataMarker, format.Dim, useColor)
			fmt.Printf("%s%-10s  %s  %-6s  %-14s  %-10s  %-14s  %-10s  %s\n",
				marker, nameStr, tierStr, authStr, noteStr, "", "", "", row.note)
			continue
		}

		fiveBar := buildStatusBar(row.fivePct, useColor)
		sevenBar := buildStatusBar(row.sevenPct, useColor)
		noteStr := row.note
		if row.isStale {
			noteStr = format.Colorize(row.note, format.Dim, useColor)
		}

		fmt.Printf("%s%-10s  %s  %-6s  %-14s  %-10s  %-14s  %-10s  %s\n",
			marker, nameStr, tierStr, authStr,
			fiveBar, row.fiveResetStr,
			sevenBar, row.sevenResetStr,
			noteStr)
	}
}

// buildStatusBar returns a formatted bar+percentage string, colored if enabled.
func buildStatusBar(pct float64, useColor bool) string {
	bar := format.ProgressBar(pct, 5)
	pctStr := fmt.Sprintf("%3.0f%%", pct)
	if useColor {
		col := format.UsageColor(pct)
		return format.Colorize(bar, col, true) + " " + format.Colorize(pctStr, col, true)
	}
	return fmt.Sprintf("%s %s", bar, pctStr)
}

func printRecommendation(rows []statusRow, allStale bool, useColor bool) {
	var best *statusRow
	for i := range rows {
		r := &rows[i]
		if !r.hasData {
			continue
		}
		if best == nil || r.fivePct < best.fivePct {
			best = r
		}
	}

	fmt.Println()
	if best != nil {
		rec := fmt.Sprintf("✓ Recommended: %s (lowest 5h usage at %.0f%%)", best.name, best.fivePct)
		fmt.Println(format.Colorize(rec, format.Green+format.Bold, useColor))
	}

	if allStale {
		fmt.Println("Run a session to refresh rate limit data.")
	}
}
