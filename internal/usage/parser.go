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
	Model       string
	Usage       TokenUsage
	Timestamp   time.Time
	SessionID   string
	ProjectPath string // project directory name extracted from JSONL file path
	IsSubagent  bool   // true if parsed from a subagents/ directory
	CLITool     string // "claude-code", "codex", "amp", "opencode"
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

// extractPathMeta derives the project directory name and subagent status
// from a JSONL file path relative to the projects root.
// Example paths:
//
//	"I--google_drive-homebase/abc123.jsonl"           → project="I--google_drive-homebase", subagent=false
//	"I--google_drive-homebase/abc123/subagents/x.jsonl" → project="I--google_drive-homebase", subagent=true
func extractPathMeta(relPath string) (project string, isSubagent bool) {
	// Normalize to forward slashes.
	rel := filepath.ToSlash(relPath)
	parts := strings.Split(rel, "/")
	if len(parts) > 0 {
		project = parts[0]
	}
	isSubagent = strings.Contains(rel, "/subagents/")
	return
}

// ScanDir walks <configDir>/projects/**/*.jsonl and calls ParseFile on each.
// Includes subagent directories (they contain real token usage).
// Populates ProjectPath and IsSubagent on each Entry from the file path.
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

		relPath, _ := filepath.Rel(root, path)
		project, isSubagent := extractPathMeta(relPath)

		// Per-file errors are non-fatal: skip and continue.
		_ = ParseFile(path, func(e Entry) {
			e.ProjectPath = project
			e.IsSubagent = isSubagent
			e.CLITool = "claude-code"
			fn(e)
		})
		return nil
	})
}

// cliToolDirs returns candidate root directories for each supported CLI tool.
// Each entry is (toolName, dirPath). Non-existent paths are included; callers
// should call ScanDir per tool and skip errors gracefully.
func cliToolDirs(home string) []struct {
	tool string
	dir  string
} {
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		xdgConfig = filepath.Join(home, ".config")
	}

	return []struct {
		tool string
		dir  string
	}{
		{"codex", filepath.Join(home, ".codex")},
		{"codex", filepath.Join(xdgConfig, "codex")},
		{"amp", filepath.Join(home, ".amp")},
		{"amp", filepath.Join(xdgConfig, "amp")},
		{"opencode", filepath.Join(home, ".opencode")},
		{"opencode", filepath.Join(xdgConfig, "opencode")},
	}
}

// ScanAllCLIs scans the Claude Code config directory plus all known other CLI
// tool directories (Codex, Amp, OpenCode). For non-Claude tools it tries the
// same JSONL format; lines that do not parse are silently skipped.
// Each Entry has CLITool set to the originating tool name.
func ScanAllCLIs(claudeConfigDir string, fn func(Entry)) error {
	// Scan Claude Code first (the canonical source).
	if err := ScanDir(claudeConfigDir, fn); err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil // cannot expand home; skip other tools silently
	}

	seen := make(map[string]bool) // avoid double-scanning the same directory
	for _, candidate := range cliToolDirs(home) {
		if seen[candidate.dir] {
			continue
		}
		seen[candidate.dir] = true

		tool := candidate.tool
		dir := candidate.dir

		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		// Walk the entire directory tree looking for JSONL files.
		// Unlike ScanDir we do NOT require a "projects/" sub-directory because
		// each tool organises its state differently.
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".jsonl") {
				return nil
			}

			_ = ParseFile(path, func(e Entry) {
				e.CLITool = tool
				fn(e)
			})
			return nil
		})
	}

	return nil
}

// ScanDirCached walks the same directory tree as ScanDir but uses the cache
// to skip unchanged files. Changed files are parsed from their cached offset
// (or from zero if new/truncated). The cache is updated in place.
// Populates ProjectPath and IsSubagent on each Entry from the file path.
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

		project, isSubagent := extractPathMeta(relPath)

		// Determine starting offset. If the file is new or was truncated,
		// parse from the beginning. Otherwise resume from the cached offset.
		var startOffset int64
		if fs, ok := cache.Files[relPath]; ok && info.Size() >= fs.Offset {
			startOffset = fs.Offset
		}

		newOffset, parseErr := ParseFileFrom(path, startOffset, func(e Entry) {
			e.ProjectPath = project
			e.IsSubagent = isSubagent
			fn(e)
		})
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
