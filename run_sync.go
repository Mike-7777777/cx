package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Mike-7777777/cx/internal/platform"
	"github.com/natefinch/atomic"
)

var syncFileList = []string{
	"settings.json", "CLAUDE.md",
	"plugins/installed_plugins.json", "plugins/config.json",
	"plugins/blocklist.json", "plugins/known_marketplaces.json",
}

// syncCmd implements Runner for the "sync" subcommand.
type syncCmd struct{}

// Run syncs config files from the main account to all secondary accounts.
func (c *syncCmd) Run(_ context.Context, app *App, args []string) error {
	flags, _ := parseFlags(args, "force")
	_, force := flags["force"]

	reg := app.Registry

	mainDir, err := reg.ResolveConfigDir(reg.Main)
	if err != nil {
		return fmt.Errorf("resolving main account %q: %v", reg.Main, err)
	}

	for name, acc := range reg.Accounts {
		if name == reg.Main {
			continue
		}
		targetDir := acc.ConfigDir
		if targetDir == "" {
			continue
		}
		fmt.Fprintf(app.Stderr, "syncing %q → %q\n", name, targetDir)
		if err := syncFiles(mainDir, targetDir, force); err != nil {
			return err
		}
	}
	return nil
}

// syncFiles copies each file in syncFileList from srcDir to dstDir,
// then syncs memory, teams, and MCP OAuth tokens.
// Missing source files are silently skipped.
// When force is false and the destination file is newer than the source,
// the user is prompted to confirm the overwrite. When force is true, the
// overwrite proceeds without prompting (used by auto-sync in switch/init).
func syncFiles(srcDir, dstDir string, force bool) error {
	reader := bufio.NewReader(os.Stdin)

	// Track files that are newer in dst for bidirectional sync.
	var newerInDst []string

	for _, rel := range syncFileList {
		src := filepath.Join(srcDir, rel)
		srcInfo, err := os.Stat(src)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("stating %q: %w", src, err)
		}

		dst := filepath.Join(dstDir, rel)

		// Conflict detection: if destination is newer than source, warn and
		// optionally prompt (interactive) or silently overwrite (force).
		if !force {
			dstInfo, statErr := os.Stat(dst)
			if statErr == nil && dstInfo.ModTime().After(srcInfo.ModTime()) {
				newerInDst = append(newerInDst, rel)

				srcBase := filepath.Base(srcDir)
				dstBase := filepath.Base(dstDir)
				fmt.Fprintf(os.Stderr, "[cx] %s in %s is newer than %s.\n", rel, dstBase, srcBase)
				fmt.Fprintf(os.Stderr, "  %s: %s (%.1fKB)\n",
					dstBase,
					dstInfo.ModTime().Format("2006-01-02 15:04:05"),
					float64(dstInfo.Size())/1024,
				)
				fmt.Fprintf(os.Stderr, "  %s: %s (%.1fKB)\n",
					srcBase,
					srcInfo.ModTime().Format("2006-01-02 15:04:05"),
					float64(srcInfo.Size())/1024,
				)
				fmt.Fprintf(os.Stderr, "  Overwrite %s with %s? [y/N] ", dstBase, srcBase)

				line, _ := reader.ReadString('\n')
				line = strings.TrimSpace(line)
				if line != "y" && line != "Y" {
					fmt.Fprintf(os.Stderr, "  skipped %s\n", rel)
					continue
				}
			}
		}

		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("reading %q: %w", src, err)
		}

		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return fmt.Errorf("creating parent for %q: %w", dst, err)
		}
		if err := os.WriteFile(dst, data, 0o600); err != nil {
			return fmt.Errorf("writing %q: %w", dst, err)
		}
		fmt.Fprintf(os.Stderr, "  synced %s\n", rel)
	}

	// Sync project memory files from main to secondary.
	if err := syncMemory(srcDir, dstDir); err != nil {
		return fmt.Errorf("syncing memory: %w", err)
	}

	// Sync teams directory from main to secondary.
	if err := syncTeams(srcDir, dstDir); err != nil {
		return fmt.Errorf("syncing teams: %w", err)
	}

	// Sync MCP OAuth tokens from main to secondary.
	if err := syncMcpOAuth(srcDir, dstDir); err != nil {
		return fmt.Errorf("syncing MCP OAuth: %w", err)
	}

	// Bidirectional sync: offer to copy newer files back to main.
	if !force && len(newerInDst) > 0 {
		dstBase := filepath.Base(dstDir)
		fmt.Fprintf(os.Stderr, "\n[cx] %s has newer config files: %s\n",
			dstBase, strings.Join(newerInDst, ", "))
		fmt.Fprintf(os.Stderr, "  Sync back to main? [y/N] ")

		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "y" || line == "Y" {
			for _, rel := range newerInDst {
				src := filepath.Join(dstDir, rel)
				dst := filepath.Join(srcDir, rel)

				data, err := os.ReadFile(src)
				if err != nil {
					return fmt.Errorf("reading %q for reverse sync: %w", src, err)
				}
				if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
					return fmt.Errorf("creating parent for %q: %w", dst, err)
				}
				if err := os.WriteFile(dst, data, 0o600); err != nil {
					return fmt.Errorf("writing %q: %w", dst, err)
				}
				fmt.Fprintf(os.Stderr, "  reverse-synced %s\n", rel)
			}
		}
	}

	return nil
}

// syncMemory copies project memory files (projects/*/memory/MEMORY.md)
// from srcDir to dstDir. Only syncs memory dirs that exist in the main account.
// Silently skips if projects/ doesn't exist in srcDir.
func syncMemory(srcDir, dstDir string) error {
	projectsDir := filepath.Join(srcDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading projects dir %q: %w", projectsDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		memFile := filepath.Join(projectsDir, entry.Name(), "memory", "MEMORY.md")
		if _, err := os.Stat(memFile); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return fmt.Errorf("stating %q: %w", memFile, err)
		}

		data, err := os.ReadFile(memFile)
		if err != nil {
			return fmt.Errorf("reading %q: %w", memFile, err)
		}

		dstFile := filepath.Join(dstDir, "projects", entry.Name(), "memory", "MEMORY.md")
		if err := os.MkdirAll(filepath.Dir(dstFile), 0o700); err != nil {
			return fmt.Errorf("creating parent for %q: %w", dstFile, err)
		}
		if err := os.WriteFile(dstFile, data, 0o600); err != nil {
			return fmt.Errorf("writing %q: %w", dstFile, err)
		}

		rel := filepath.Join("projects", entry.Name(), "memory", "MEMORY.md")
		fmt.Fprintf(os.Stderr, "  synced %s\n", rel)
	}
	return nil
}

// syncTeams copies the entire teams/ directory tree from srcDir to dstDir.
// Silently skips if teams/ doesn't exist in srcDir.
func syncTeams(srcDir, dstDir string) error {
	teamsDir := filepath.Join(srcDir, "teams")
	if _, err := os.Stat(teamsDir); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("stating teams dir %q: %w", teamsDir, err)
	}

	dstTeams := filepath.Join(dstDir, "teams")
	if err := platform.CopyDir(teamsDir, dstTeams); err != nil {
		return fmt.Errorf("copying teams dir: %w", err)
	}
	fmt.Fprintf(os.Stderr, "  synced teams/\n")
	return nil
}

// syncMcpOAuth merges the mcpOAuth key from src's .credentials.json into dst's
// .credentials.json. The claudeAiOauth key is account-specific and never synced.
// Only syncs if src has mcpOAuth and dst doesn't.
// Writes atomically to prevent corruption.
func syncMcpOAuth(srcDir, dstDir string) error {
	const credFile = ".credentials.json"

	srcPath := filepath.Join(srcDir, credFile)
	srcData, err := os.ReadFile(srcPath)
	if os.IsNotExist(err) {
		return nil // no credentials in source
	}
	if err != nil {
		return fmt.Errorf("reading src credentials %q: %w", srcPath, err)
	}

	var srcCreds map[string]json.RawMessage
	if err := json.Unmarshal(srcData, &srcCreds); err != nil {
		return nil // corrupt source, skip silently
	}

	srcOAuth, hasSrcOAuth := srcCreds["mcpOAuth"]
	if !hasSrcOAuth {
		return nil // nothing to sync
	}

	dstPath := filepath.Join(dstDir, credFile)
	dstData, err := os.ReadFile(dstPath)
	if os.IsNotExist(err) {
		// dst has no credentials yet; create with only mcpOAuth.
		dstCreds := map[string]json.RawMessage{
			"mcpOAuth": srcOAuth,
		}
		return writeCredentialsAtomic(dstPath, dstCreds)
	}
	if err != nil {
		return fmt.Errorf("reading dst credentials %q: %w", dstPath, err)
	}

	var dstCreds map[string]json.RawMessage
	if err := json.Unmarshal(dstData, &dstCreds); err != nil {
		return nil // corrupt destination, skip silently
	}

	// Only sync if dst doesn't have mcpOAuth.
	if _, hasDstOAuth := dstCreds["mcpOAuth"]; hasDstOAuth {
		return nil // dst already has mcpOAuth
	}

	dstCreds["mcpOAuth"] = srcOAuth
	if err := writeCredentialsAtomic(dstPath, dstCreds); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "  synced mcpOAuth tokens\n")
	return nil
}

// writeCredentialsAtomic writes a credentials map to path atomically.
func writeCredentialsAtomic(path string, creds map[string]json.RawMessage) error {
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling credentials: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating parent for %q: %w", path, err)
	}

	if err := atomic.WriteFile(path, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("atomic write %q: %w", path, err)
	}
	return nil
}
