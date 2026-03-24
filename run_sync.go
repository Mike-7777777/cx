package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MasaYan24/cc-monitor/internal/config"
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

	for name, acc := range reg.Accounts {
		if name == reg.Primary {
			continue
		}
		targetDir := acc.ConfigDir
		if targetDir == "" {
			continue
		}
		fmt.Printf("syncing %q → %q\n", name, targetDir)
		if err := syncFiles(primaryDir, targetDir); err != nil {
			fmt.Fprintf(os.Stderr, "cc-monitor sync: %v\n", err)
			os.Exit(1)
		}
	}
}

// syncFiles copies each file in syncFileList from srcDir to dstDir.
// Missing source files are silently skipped.
func syncFiles(srcDir, dstDir string) error {
	for _, rel := range syncFileList {
		src := filepath.Join(srcDir, rel)
		data, err := os.ReadFile(src)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("reading %q: %w", src, err)
		}

		dst := filepath.Join(dstDir, rel)
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
