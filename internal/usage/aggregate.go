package usage

import (
	"fmt"
	"sort"
	"time"
)

// UsageSummary holds aggregated token counts and cost for a group of entries.
type UsageSummary struct {
	InputTokens              int64   `json:"InputTokens"`
	OutputTokens             int64   `json:"OutputTokens"`
	CacheCreationInputTokens int64   `json:"CacheCreationInputTokens"`
	CacheReadInputTokens     int64   `json:"CacheReadInputTokens"`
	TotalTokens              int64   `json:"TotalTokens"`
	CostUSD                  float64 `json:"CostUSD"`
	EntryCount               int     `json:"EntryCount"`
}

// DailyReport summarizes usage for a single UTC day.
type DailyReport struct {
	Date    string                   `json:"Date"`
	Summary UsageSummary             `json:"Summary"`
	Models  map[string]*UsageSummary `json:"Models"`
}

// SessionReport summarizes usage for a single session.
type SessionReport struct {
	SessionID string                   `json:"SessionID"`
	StartTime time.Time                `json:"StartTime"`
	EndTime   time.Time                `json:"EndTime"`
	Summary   UsageSummary             `json:"Summary"`
	Models    map[string]*UsageSummary `json:"Models"`
}

// BlockReport summarizes usage for a 5-hour time block.
type BlockReport struct {
	StartTime time.Time                `json:"StartTime"`
	EndTime   time.Time                `json:"EndTime"`
	Summary   UsageSummary             `json:"Summary"`
	Models    map[string]*UsageSummary `json:"Models"`
}

// addEntry accumulates an entry's tokens and cost into a UsageSummary.
func addEntry(s *UsageSummary, e Entry) {
	s.InputTokens += e.Usage.InputTokens
	s.OutputTokens += e.Usage.OutputTokens
	s.CacheCreationInputTokens += e.Usage.CacheCreationInputTokens
	s.CacheReadInputTokens += e.Usage.CacheReadInputTokens
	s.TotalTokens += e.Usage.InputTokens + e.Usage.OutputTokens +
		e.Usage.CacheCreationInputTokens + e.Usage.CacheReadInputTokens
	s.CostUSD += CalculateCost(e.Model, e.Usage)
	s.EntryCount++
}

// getOrCreateModel returns the per-model summary, creating it if needed.
func getOrCreateModel(models map[string]*UsageSummary, model string) *UsageSummary {
	s, ok := models[model]
	if !ok {
		s = &UsageSummary{}
		models[model] = s
	}
	return s
}

// AggregateDailies groups entries by UTC date. Returns sorted by date ascending.
func AggregateDailies(entries []Entry) []DailyReport {
	byDate := make(map[string]*DailyReport)

	for _, e := range entries {
		dateKey := e.Timestamp.UTC().Format("2006-01-02")
		dr, ok := byDate[dateKey]
		if !ok {
			dr = &DailyReport{
				Date:   dateKey,
				Models: make(map[string]*UsageSummary),
			}
			byDate[dateKey] = dr
		}
		addEntry(&dr.Summary, e)
		addEntry(getOrCreateModel(dr.Models, e.Model), e)
	}

	reports := make([]DailyReport, 0, len(byDate))
	for _, dr := range byDate {
		reports = append(reports, *dr)
	}
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Date < reports[j].Date
	})
	return reports
}

// AggregateSessions groups entries by session ID. Returns sorted by start time.
func AggregateSessions(entries []Entry) []SessionReport {
	bySession := make(map[string]*SessionReport)

	for _, e := range entries {
		sr, ok := bySession[e.SessionID]
		if !ok {
			sr = &SessionReport{
				SessionID: e.SessionID,
				StartTime: e.Timestamp,
				EndTime:   e.Timestamp,
				Models:    make(map[string]*UsageSummary),
			}
			bySession[e.SessionID] = sr
		}
		if e.Timestamp.Before(sr.StartTime) {
			sr.StartTime = e.Timestamp
		}
		if e.Timestamp.After(sr.EndTime) {
			sr.EndTime = e.Timestamp
		}
		addEntry(&sr.Summary, e)
		addEntry(getOrCreateModel(sr.Models, e.Model), e)
	}

	reports := make([]SessionReport, 0, len(bySession))
	for _, sr := range bySession {
		reports = append(reports, *sr)
	}
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].StartTime.Before(reports[j].StartTime)
	})
	return reports
}

// blockStart calculates the start of the 5-hour block containing t.
// Blocks are aligned to midnight UTC: [00:00-05:00), [05:00-10:00), [10:00-15:00), etc.
func blockStart(t time.Time) time.Time {
	utc := t.UTC()
	hour := utc.Hour()
	blockHour := (hour / 5) * 5
	return time.Date(utc.Year(), utc.Month(), utc.Day(), blockHour, 0, 0, 0, time.UTC)
}

// MonthlyReport summarizes usage for a single UTC month.
type MonthlyReport struct {
	Month   string                   `json:"Month"` // "2026-03"
	Summary UsageSummary             `json:"Summary"`
	Models  map[string]*UsageSummary `json:"Models"`
}

// WeeklyReport summarizes usage for a single ISO week.
type WeeklyReport struct {
	Week    string                   `json:"Week"` // "2026-W13"
	Summary UsageSummary             `json:"Summary"`
	Models  map[string]*UsageSummary `json:"Models"`
}

// AggregateMonthly groups entries by UTC month. Returns sorted by month ascending.
func AggregateMonthly(entries []Entry) []MonthlyReport {
	byMonth := make(map[string]*MonthlyReport)

	for _, e := range entries {
		monthKey := e.Timestamp.UTC().Format("2006-01")
		mr, ok := byMonth[monthKey]
		if !ok {
			mr = &MonthlyReport{
				Month:  monthKey,
				Models: make(map[string]*UsageSummary),
			}
			byMonth[monthKey] = mr
		}
		addEntry(&mr.Summary, e)
		addEntry(getOrCreateModel(mr.Models, e.Model), e)
	}

	reports := make([]MonthlyReport, 0, len(byMonth))
	for _, mr := range byMonth {
		reports = append(reports, *mr)
	}
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Month < reports[j].Month
	})
	return reports
}

// AggregateWeekly groups entries by ISO week. Returns sorted by week ascending.
func AggregateWeekly(entries []Entry) []WeeklyReport {
	byWeek := make(map[string]*WeeklyReport)

	for _, e := range entries {
		y, w := e.Timestamp.UTC().ISOWeek()
		weekKey := fmt.Sprintf("%d-W%02d", y, w)
		wr, ok := byWeek[weekKey]
		if !ok {
			wr = &WeeklyReport{
				Week:   weekKey,
				Models: make(map[string]*UsageSummary),
			}
			byWeek[weekKey] = wr
		}
		addEntry(&wr.Summary, e)
		addEntry(getOrCreateModel(wr.Models, e.Model), e)
	}

	reports := make([]WeeklyReport, 0, len(byWeek))
	for _, wr := range byWeek {
		reports = append(reports, *wr)
	}
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Week < reports[j].Week
	})
	return reports
}

// ProjectReport summarizes usage for a single project directory.
type ProjectReport struct {
	Project string       `json:"Project"`
	Summary UsageSummary `json:"Summary"`
}

// AggregateByProject groups entries by their ProjectPath.
// Returns sorted by cost descending (heaviest project first).
func AggregateByProject(entries []Entry) []ProjectReport {
	byProject := make(map[string]*UsageSummary)

	for _, e := range entries {
		proj := e.ProjectPath
		if proj == "" {
			proj = "(unknown)"
		}
		s, ok := byProject[proj]
		if !ok {
			s = &UsageSummary{}
			byProject[proj] = s
		}
		addEntry(s, e)
	}

	reports := make([]ProjectReport, 0, len(byProject))
	for proj, s := range byProject {
		reports = append(reports, ProjectReport{Project: proj, Summary: *s})
	}
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Summary.CostUSD > reports[j].Summary.CostUSD
	})
	return reports
}

// TrendPair holds current and previous period values for trend comparison.
type TrendPair struct {
	Date            string  `json:"Date"`
	CurrentTokens   int64   `json:"CurrentTokens"`
	PreviousTokens  int64   `json:"PreviousTokens"`
	CurrentCost     float64 `json:"CurrentCost"`
	PreviousCost    float64 `json:"PreviousCost"`
	ChangePercent   float64 `json:"ChangePercent"`   // token change %
	CostChangePct   float64 `json:"CostChangePct"`   // cost change %
}

// AggregateTrend compares the current period (since to now) with the previous
// period of equal length. Returns day-by-day pairs plus an overall total pair.
// If since is zero, defaults to 7 days ago.
func AggregateTrend(entries []Entry, since time.Time) []TrendPair {
	now := time.Now().UTC()
	if since.IsZero() {
		since = now.AddDate(0, 0, -7)
	}
	since = since.UTC()

	// Current period: [since, now]
	// Period length in days.
	days := int(now.Sub(since).Hours()/24) + 1
	if days < 1 {
		days = 1
	}

	// Previous period: [since - days, since - 1 day]
	prevStart := since.AddDate(0, 0, -days)

	// Build daily maps for both periods.
	currentByDay := make(map[string]UsageSummary)
	previousByDay := make(map[string]UsageSummary)

	for _, e := range entries {
		ts := e.Timestamp.UTC()
		dateKey := ts.Format("2006-01-02")

		if !ts.Before(since) && !ts.After(now) {
			s := currentByDay[dateKey]
			addEntry(&s, e)
			currentByDay[dateKey] = s
		} else if !ts.Before(prevStart) && ts.Before(since) {
			s := previousByDay[dateKey]
			addEntry(&s, e)
			previousByDay[dateKey] = s
		}
	}

	// Build day-indexed pairs.
	var pairs []TrendPair
	var totalCurrent, totalPrevious int64
	var totalCostCurrent, totalCostPrevious float64

	for i := 0; i < days; i++ {
		curDate := since.AddDate(0, 0, i).Format("2006-01-02")
		prevDate := prevStart.AddDate(0, 0, i).Format("2006-01-02")

		cur := currentByDay[curDate]
		prev := previousByDay[prevDate]

		var changePct, costChangePct float64
		if prev.TotalTokens > 0 {
			changePct = float64(cur.TotalTokens-prev.TotalTokens) / float64(prev.TotalTokens) * 100
		}
		if prev.CostUSD > 0 {
			costChangePct = (cur.CostUSD - prev.CostUSD) / prev.CostUSD * 100
		}

		pairs = append(pairs, TrendPair{
			Date:           curDate,
			CurrentTokens:  cur.TotalTokens,
			PreviousTokens: prev.TotalTokens,
			CurrentCost:    cur.CostUSD,
			PreviousCost:   prev.CostUSD,
			ChangePercent:  changePct,
			CostChangePct:  costChangePct,
		})

		totalCurrent += cur.TotalTokens
		totalPrevious += prev.TotalTokens
		totalCostCurrent += cur.CostUSD
		totalCostPrevious += prev.CostUSD
	}

	// Append a total row.
	var totalChange, totalCostChange float64
	if totalPrevious > 0 {
		totalChange = float64(totalCurrent-totalPrevious) / float64(totalPrevious) * 100
	}
	if totalCostPrevious > 0 {
		totalCostChange = (totalCostCurrent - totalCostPrevious) / totalCostPrevious * 100
	}
	pairs = append(pairs, TrendPair{
		Date:           "Total",
		CurrentTokens:  totalCurrent,
		PreviousTokens: totalPrevious,
		CurrentCost:    totalCostCurrent,
		PreviousCost:   totalCostPrevious,
		ChangePercent:  totalChange,
		CostChangePct:  totalCostChange,
	})

	return pairs
}

// SubagentBreakdown holds the main vs subagent cost split.
type SubagentBreakdown struct {
	MainCost       float64 `json:"MainCost"`
	SubagentCost   float64 `json:"SubagentCost"`
	MainTokens     int64   `json:"MainTokens"`
	SubagentTokens int64   `json:"SubagentTokens"`
	MainCount      int     `json:"MainCount"`
	SubagentCount  int     `json:"SubagentCount"`
}

// AggregateSubagents splits entries into main session vs subagent totals.
func AggregateSubagents(entries []Entry) SubagentBreakdown {
	var b SubagentBreakdown
	for _, e := range entries {
		cost := CalculateCost(e.Model, e.Usage)
		total := e.Usage.InputTokens + e.Usage.OutputTokens +
			e.Usage.CacheCreationInputTokens + e.Usage.CacheReadInputTokens
		if e.IsSubagent {
			b.SubagentCost += cost
			b.SubagentTokens += total
			b.SubagentCount++
		} else {
			b.MainCost += cost
			b.MainTokens += total
			b.MainCount++
		}
	}
	return b
}

// AggregateBlocks groups entries into 5-hour blocks aligned to midnight UTC.
// Returns sorted by block start time.
func AggregateBlocks(entries []Entry) []BlockReport {
	byBlock := make(map[time.Time]*BlockReport)

	for _, e := range entries {
		bs := blockStart(e.Timestamp)
		br, ok := byBlock[bs]
		if !ok {
			br = &BlockReport{
				StartTime: bs,
				EndTime:   bs.Add(5 * time.Hour),
				Models:    make(map[string]*UsageSummary),
			}
			byBlock[bs] = br
		}
		addEntry(&br.Summary, e)
		addEntry(getOrCreateModel(br.Models, e.Model), e)
	}

	reports := make([]BlockReport, 0, len(byBlock))
	for _, br := range byBlock {
		reports = append(reports, *br)
	}
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].StartTime.Before(reports[j].StartTime)
	})
	return reports
}
