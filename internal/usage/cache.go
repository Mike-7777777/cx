package usage

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/natefinch/atomic"
)

// FileState tracks the last-processed state of a single JSONL file.
type FileState struct {
	Size      int64 `json:"size"`
	MtimeUnix int64 `json:"mtime_unix"`
	Offset    int64 `json:"offset"` // byte offset where we stopped reading
}

// CachedSummary holds aggregated token counts and cost for a cached period.
type CachedSummary struct {
	InputTokens              int64                    `json:"input_tokens"`
	OutputTokens             int64                    `json:"output_tokens"`
	CacheCreationInputTokens int64                    `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64                    `json:"cache_read_input_tokens"`
	TotalTokens              int64                    `json:"total_tokens"`
	CostUSD                  float64                  `json:"cost_usd"`
	EntryCount               int                      `json:"entry_count"`
	Models                   map[string]CachedSummary `json:"models,omitempty"`
}

// UsageCache stores incremental scan state and cached daily aggregates.
type UsageCache struct {
	Version int                      `json:"version"`
	Files   map[string]FileState     `json:"files"`
	Daily   map[string]CachedSummary `json:"daily"`
	path    string
}

const cacheVersion = 1

// LoadUsageCache loads the cache from path, or returns an empty cache if the
// file does not exist or has an incompatible version.
func LoadUsageCache(path string) (*UsageCache, error) {
	c := &UsageCache{
		Version: cacheVersion,
		Files:   make(map[string]FileState),
		Daily:   make(map[string]CachedSummary),
		path:    path,
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, nil // treat read errors as cache miss
	}

	var loaded UsageCache
	if err := json.Unmarshal(data, &loaded); err != nil {
		return c, nil // corrupt cache → start fresh
	}

	if loaded.Version != cacheVersion {
		return c, nil // version mismatch → start fresh
	}

	loaded.path = path
	if loaded.Files == nil {
		loaded.Files = make(map[string]FileState)
	}
	if loaded.Daily == nil {
		loaded.Daily = make(map[string]CachedSummary)
	}
	return &loaded, nil
}

// Save writes the cache atomically to its path.
func (c *UsageCache) Save() error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return atomic.WriteFile(c.path, bytes.NewReader(data))
}

// NeedsUpdate returns true if the file should be re-parsed because it is new,
// modified, or truncated since the last scan.
func (c *UsageCache) NeedsUpdate(relPath string, info os.FileInfo) bool {
	fs, ok := c.Files[relPath]
	if !ok {
		return true // new file
	}
	if info.Size() != fs.Size {
		return true // size changed
	}
	if info.ModTime().Unix() != fs.MtimeUnix {
		return true // mtime changed
	}
	if info.Size() < fs.Offset {
		return true // truncated
	}
	return false
}

// MergeDailyEntry adds an entry's data to the cached daily summary.
func (c *UsageCache) MergeDailyEntry(dateKey string, e Entry) {
	ds := c.Daily[dateKey]

	ds.InputTokens += e.Usage.InputTokens
	ds.OutputTokens += e.Usage.OutputTokens
	ds.CacheCreationInputTokens += e.Usage.CacheCreationInputTokens
	ds.CacheReadInputTokens += e.Usage.CacheReadInputTokens
	ds.TotalTokens += e.Usage.InputTokens + e.Usage.OutputTokens +
		e.Usage.CacheCreationInputTokens + e.Usage.CacheReadInputTokens
	ds.CostUSD += CalculateCost(e.Model, e.Usage)
	ds.EntryCount++

	// Per-model breakdown.
	if ds.Models == nil {
		ds.Models = make(map[string]CachedSummary)
	}
	ms := ds.Models[e.Model]
	ms.InputTokens += e.Usage.InputTokens
	ms.OutputTokens += e.Usage.OutputTokens
	ms.CacheCreationInputTokens += e.Usage.CacheCreationInputTokens
	ms.CacheReadInputTokens += e.Usage.CacheReadInputTokens
	ms.TotalTokens += e.Usage.InputTokens + e.Usage.OutputTokens +
		e.Usage.CacheCreationInputTokens + e.Usage.CacheReadInputTokens
	ms.CostUSD += CalculateCost(e.Model, e.Usage)
	ms.EntryCount++
	ds.Models[e.Model] = ms

	c.Daily[dateKey] = ds
}

// DailyReports converts the cached daily map to sorted DailyReport slices.
func (c *UsageCache) DailyReports() []DailyReport {
	reports := make([]DailyReport, 0, len(c.Daily))
	for dateKey, cs := range c.Daily {
		dr := DailyReport{
			Date: dateKey,
			Summary: UsageSummary{
				InputTokens:              cs.InputTokens,
				OutputTokens:             cs.OutputTokens,
				CacheCreationInputTokens: cs.CacheCreationInputTokens,
				CacheReadInputTokens:     cs.CacheReadInputTokens,
				TotalTokens:              cs.TotalTokens,
				CostUSD:                  cs.CostUSD,
				EntryCount:               cs.EntryCount,
			},
			Models: make(map[string]*UsageSummary),
		}
		for model, mcs := range cs.Models {
			dr.Models[model] = &UsageSummary{
				InputTokens:              mcs.InputTokens,
				OutputTokens:             mcs.OutputTokens,
				CacheCreationInputTokens: mcs.CacheCreationInputTokens,
				CacheReadInputTokens:     mcs.CacheReadInputTokens,
				TotalTokens:              mcs.TotalTokens,
				CostUSD:                  mcs.CostUSD,
				EntryCount:               mcs.EntryCount,
			}
		}
		reports = append(reports, dr)
	}
	return reports
}
