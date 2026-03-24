package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/MasaYan24/cc-monitor/internal/cache"
	"github.com/MasaYan24/cc-monitor/internal/config"
	"github.com/MasaYan24/cc-monitor/internal/format"
)

const (
	staleThreshold    = 10 * time.Minute
	allStaleThreshold = 1 * time.Hour
	noDataMarker      = "no data"
	stalePrefix       = "stale"
)

type statusRow struct {
	name         string
	fivePct      float64
	fiveResetStr string
	sevenPct     float64
	note         string
	hasData      bool
	isStale      bool
}

func runStatus() {
	regPath, err := config.RegistryPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cc-monitor status: %v\n", err)
		os.Exit(1)
	}

	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cc-monitor status: %v\n", err)
		os.Exit(1)
	}

	if len(reg.Accounts) == 0 {
		fmt.Println("No accounts configured. Run: cc-monitor init <name>")
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
		rows = append(rows, row)

		if row.hasData && !row.isStale {
			allStale = false
		}
	}

	printTable(rows)
	printRecommendation(rows, allStale)
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
	}

	return row
}

func printTable(rows []statusRow) {
	header := fmt.Sprintf("%-10s  %-14s  %-12s  %-14s  %s",
		"Account", "5h Usage", "Resets In", "7d Usage", "Note")
	separator := "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

	fmt.Println(header)
	fmt.Println(separator)

	for _, row := range rows {
		if !row.hasData {
			fmt.Printf("%-10s  %-14s  %-12s  %-14s  %s\n",
				row.name, noDataMarker, "", "", row.note)
			continue
		}

		fiveBar := fmt.Sprintf("%s %3.0f%%", format.ProgressBar(row.fivePct, 5), row.fivePct)
		sevenBar := fmt.Sprintf("%s %3.0f%%", format.ProgressBar(row.sevenPct, 5), row.sevenPct)

		fmt.Printf("%-10s  %-14s  %-12s  %-14s  %s\n",
			row.name, fiveBar, row.fiveResetStr, sevenBar, row.note)
	}
}

func printRecommendation(rows []statusRow, allStale bool) {
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
		fmt.Printf("✓ Recommended: %s (lowest 5h usage at %.0f%%)\n", best.name, best.fivePct)
	}

	if allStale {
		fmt.Println("Run a session to refresh rate limit data.")
	}
}
