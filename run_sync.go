package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Mike-7777777/cc-monitor/internal/config"
)

var syncFileList = []string{
	"settings.json", "CLAUDE.md",
	"plugins/installed_plugins.json", "plugins/config.json",
	"plugins/blocklist.json", "plugins/known_marketplaces.json",
}

func runSync() {
	regPath, err := config.RegistryPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cc-monitor sync: %v\n", err)
		os.Exit(1)
	}

	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cc-monitor sync: %v\n", err)
		os.Exit(1)
	}

	primaryDir, err := reg.ResolveConfigDir(reg.Primary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cc-monitor sync: resolving primary account %q: %v\n", reg.Primary, err)
		os.Exit(1)
	}

	force := hasFlag("--force")

	for name, acc := range reg.Accounts {
		if name == reg.Primary {
			continue
		}
		targetDir := acc.ConfigDir
		if targetDir == "" {
			continue
		}
		fmt.Printf("syncing %q → %q\n", name, targetDir)
		if err := syncFiles(primaryDir, targetDir, force); err != nil {
			fmt.Fprintf(os.Stderr, "cc-monitor sync: %v\n", err)
			os.Exit(1)
		}
	}
}

// syncFiles copies each file in syncFileList from srcDir to dstDir.
// Missing source files are silently skipped.
// When force is false and the destination file is newer than the source,
// the user is prompted to confirm the overwrite. When force is true, the
// overwrite proceeds without prompting (used by auto-sync in switch/init).
func syncFiles(srcDir, dstDir string, force bool) error {
	reader := bufio.NewReader(os.Stdin)

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
				srcBase := filepath.Base(srcDir)
				dstBase := filepath.Base(dstDir)
				fmt.Printf("[cc-monitor] %s in %s is newer than %s.\n", rel, dstBase, srcBase)
				fmt.Printf("  %s: %s (%.1fKB)\n",
					dstBase,
					dstInfo.ModTime().Format("2006-01-02 15:04:05"),
					float64(dstInfo.Size())/1024,
				)
				fmt.Printf("  %s: %s (%.1fKB)\n",
					srcBase,
					srcInfo.ModTime().Format("2006-01-02 15:04:05"),
					float64(srcInfo.Size())/1024,
				)
				fmt.Printf("  Overwrite %s with %s? [y/N] ", dstBase, srcBase)

				line, _ := reader.ReadString('\n')
				line = strings.TrimSpace(line)
				if line != "y" && line != "Y" {
					fmt.Printf("  skipped %s\n", rel)
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
		fmt.Printf("  synced %s\n", rel)
	}
	return nil
}
