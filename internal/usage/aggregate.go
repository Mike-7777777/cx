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
