package usage

import (
	"bufio"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TokenUsage holds token counts from a single API response.
type TokenUsage struct {
	InputTokens              int64
	OutputTokens             int64
	CacheCreationInputTokens int64
	CacheReadInputTokens     int64
}

// Entry represents a single parsed assistant response with usage data.
type Entry struct {
	Model     string
	Usage     TokenUsage
	Timestamp time.Time
	SessionID string
}

// jsonEntry is the internal struct for partial JSON parsing of JSONL lines.
type jsonEntry struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	SessionID string `json:"sessionId"`
	Message   struct {
		Model string `json:"model"`
		Usage *struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// ParseFile streams a single JSONL file, calling fn for each valid assistant entry.
// Uses bufio.Reader to handle arbitrarily long lines (thinking blocks can exceed 4MB).
// Skips non-assistant lines and corrupt/truncated lines.
func ParseFile(path string, fn func(Entry)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	reader := bufio.NewReaderSize(f, 256*1024) // 256KB read buffer
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			parseLine(line, fn)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}
	return nil
}

// parseLine processes a single JSONL line, calling fn if it is a valid assistant entry.
func parseLine(line []byte, fn func(Entry)) {
	s := string(line)

	// Lightweight pre-check: skip lines that cannot be assistant entries.
	if !strings.Contains(s, `"type":"assistant"`) &&
		!strings.Contains(s, `"type": "assistant"`) {
		return
	}

	var je jsonEntry
	if err := json.Unmarshal(line, &je); err != nil {
		return
	}

	if je.Type != "assistant" {
		return
	}
	if je.Message.Usage == nil {
		return
	}

	ts, err := time.Parse(time.RFC3339Nano, je.Timestamp)
	if err != nil {
		return
	}

	fn(Entry{
		Model: je.Message.Model,
		Usage: TokenUsage{
			InputTokens:              je.Message.Usage.InputTokens,
			OutputTokens:             je.Message.Usage.OutputTokens,
			CacheCreationInputTokens: je.Message.Usage.CacheCreationInputTokens,
			CacheReadInputTokens:     je.Message.Usage.CacheReadInputTokens,
		},
		Timestamp: ts,
		SessionID: je.SessionID,
	})
}

// ParseFileFrom reads a JSONL file starting from the given byte offset,
// calling fn for each valid assistant entry. Returns the new offset (end of
// file position) after parsing. When offset is 0, it is equivalent to ParseFile.
func ParseFileFrom(path string, offset int64, fn func(Entry)) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return 0, err
		}
	}

	reader := bufio.NewReaderSize(f, 256*1024)
	pos := offset
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			pos += int64(len(line))
			parseLine(line, fn)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return pos, err
		}
	}
	return pos, nil
}

// ScanDir walks <configDir>/projects/**/*.jsonl and calls ParseFile on each.
// Includes subagent directories (they contain real token usage).
// Skips files that fail to parse and continues with remaining files.
func ScanDir(configDir string, fn func(Entry)) error {
	root := filepath.Join(configDir, "projects")
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip inaccessible directories/files.
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		// Per-file errors are non-fatal: skip and continue.
		_ = ParseFile(path, fn)
		return nil
	})
}

// ScanDirCached walks the same directory tree as ScanDir but uses the cache
// to skip unchanged files. Changed files are parsed from their cached offset
// (or from zero if new/truncated). The cache is updated in place.
// fn receives entries only from newly parsed data.
func ScanDirCached(configDir string, cache *UsageCache, fn func(Entry)) error {
	root := filepath.Join(configDir, "projects")
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			relPath = path
		}
		// Normalize to forward slashes for consistent cache keys.
		relPath = filepath.ToSlash(relPath)

		if !cache.NeedsUpdate(relPath, info) {
			return nil // file unchanged, skip
		}

		// Determine starting offset. If the file is new or was truncated,
		// parse from the beginning. Otherwise resume from the cached offset.
		var startOffset int64
		if fs, ok := cache.Files[relPath]; ok && info.Size() >= fs.Offset {
			startOffset = fs.Offset
		}

		newOffset, parseErr := ParseFileFrom(path, startOffset, fn)
		if parseErr != nil {
			// Non-fatal: skip this file but don't update cache for it.
			return nil
		}

		cache.Files[relPath] = FileState{
			Size:      info.Size(),
			MtimeUnix: info.ModTime().Unix(),
			Offset:    newOffset,
		}
		return nil
	})
}
