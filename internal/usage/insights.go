package usage

import "sort"

// HourlyReport summarizes usage for a single UTC hour (0–23).
type HourlyReport struct {
	Hour    int          `json:"Hour"`
	Summary UsageSummary `json:"Summary"`
}

// AggregateHourly groups entries by UTC hour of day (0–23).
// Returns a slice sorted by hour ascending.
func AggregateHourly(entries []Entry) []HourlyReport {
	byHour := make(map[int]*UsageSummary)

	for _, e := range entries {
		h := e.Timestamp.UTC().Hour()
		s, ok := byHour[h]
		if !ok {
			s = &UsageSummary{}
			byHour[h] = s
		}
		addEntry(s, e)
	}

	reports := make([]HourlyReport, 0, len(byHour))
	for h, s := range byHour {
		reports = append(reports, HourlyReport{Hour: h, Summary: *s})
	}
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Hour < reports[j].Hour
	})
	return reports
}

// ModelDistributionReport summarizes usage for a single model with percentage shares.
type ModelDistributionReport struct {
	Model        string       `json:"Model"`
	Summary      UsageSummary `json:"Summary"`
	CostPercent  float64      `json:"CostPercent"`
	TokenPercent float64      `json:"TokenPercent"`
	MsgPercent   float64      `json:"MsgPercent"`
}

// AggregateModelDistribution groups entries by model and computes percentage
// shares of total cost, tokens, and message count.
// Returns a slice sorted by cost descending (highest cost model first).
func AggregateModelDistribution(entries []Entry) []ModelDistributionReport {
	byModel := make(map[string]*UsageSummary)

	for _, e := range entries {
		s, ok := byModel[e.Model]
		if !ok {
			s = &UsageSummary{}
			byModel[e.Model] = s
		}
		addEntry(s, e)
	}

	// Compute totals for percentage calculation.
	var totalCost float64
	var totalTokens int64
	var totalMsgs int
	for _, s := range byModel {
		totalCost += s.CostUSD
		totalTokens += s.TotalTokens
		totalMsgs += s.EntryCount
	}

	reports := make([]ModelDistributionReport, 0, len(byModel))
	for model, s := range byModel {
		var costPct, tokenPct, msgPct float64
		if totalCost > 0 {
			costPct = s.CostUSD / totalCost * 100
		}
		if totalTokens > 0 {
			tokenPct = float64(s.TotalTokens) / float64(totalTokens) * 100
		}
		if totalMsgs > 0 {
			msgPct = float64(s.EntryCount) / float64(totalMsgs) * 100
		}
		reports = append(reports, ModelDistributionReport{
			Model:        model,
			Summary:      *s,
			CostPercent:  costPct,
			TokenPercent: tokenPct,
			MsgPercent:   msgPct,
		})
	}

	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Summary.CostUSD > reports[j].Summary.CostUSD
	})
	return reports
}

// EfficiencyMetrics holds derived efficiency indicators across a set of entries.
type EfficiencyMetrics struct {
	// CacheHitRatio is cache_reads / (cache_reads + cache_creates). Zero when no cache tokens.
	CacheHitRatio float64 `json:"CacheHitRatio"`
	// AvgTokensPerMsg is total tokens / entry count. Zero when no entries.
	AvgTokensPerMsg int64 `json:"AvgTokensPerMsg"`
	// AvgCostPerMsg is total cost / entry count. Zero when no entries.
	AvgCostPerMsg float64 `json:"AvgCostPerMsg"`
	// InputOutputRatio is input tokens / output tokens. Zero when no output tokens.
	InputOutputRatio float64 `json:"InputOutputRatio"`
}

// CalculateEfficiency computes efficiency metrics across all entries.
func CalculateEfficiency(entries []Entry) EfficiencyMetrics {
	var totalTokens, inputTokens, outputTokens, cacheReads, cacheCreates int64
	var totalCost float64

	for _, e := range entries {
		inputTokens += e.Usage.InputTokens
		outputTokens += e.Usage.OutputTokens
		cacheReads += e.Usage.CacheReadInputTokens
		cacheCreates += e.Usage.CacheCreationInputTokens
		totalTokens += e.Usage.InputTokens + e.Usage.OutputTokens +
			e.Usage.CacheCreationInputTokens + e.Usage.CacheReadInputTokens
		totalCost += CalculateCost(e.Model, e.Usage)
	}

	n := int64(len(entries))
	var m EfficiencyMetrics

	cacheTotal := cacheReads + cacheCreates
	if cacheTotal > 0 {
		m.CacheHitRatio = float64(cacheReads) / float64(cacheTotal)
	}
	if n > 0 {
		m.AvgTokensPerMsg = totalTokens / n
		m.AvgCostPerMsg = totalCost / float64(n)
	}
	if outputTokens > 0 {
		m.InputOutputRatio = float64(inputTokens) / float64(outputTokens)
	}

	return m
}

// FindPeakHours returns the top n hourly reports ranked by total token count
// in descending order. If n >= number of distinct hours, all hours are returned.
func FindPeakHours(entries []Entry, n int) []HourlyReport {
	all := AggregateHourly(entries)

	// Sort by total tokens descending.
	sort.Slice(all, func(i, j int) bool {
		return all[i].Summary.TotalTokens > all[j].Summary.TotalTokens
	})

	if n > len(all) {
		n = len(all)
	}
	return all[:n]
}
