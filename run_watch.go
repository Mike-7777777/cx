package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Mike-7777777/cx/internal/config"
)

const (
	watchPollInterval = 30 * time.Second
)

// watchState tracks the last-seen mtime and size of each synced file in the
// primary config directory so we can detect changes on each poll cycle.
type watchState map[string]watchFileMeta

type watchFileMeta struct {
	mtime time.Time
	size  int64
}

// runWatch implements the `cx watch` command.
// It polls the primary account's config files every 30 seconds and
// auto-syncs to all secondaries when a change is detected.
func runWatch() {
	regPath, err := config.RegistryPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cx watch: %v\n", err)
		os.Exit(1)
	}

	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cx watch: %v\n", err)
		os.Exit(1)
	}

	if reg.Primary == "" {
		fmt.Fprintln(os.Stderr, "cx watch: no primary account configured; run: cx init <name>")
		os.Exit(1)
	}

	primaryDir, err := reg.ResolveConfigDir(reg.Primary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cx watch: resolving primary account %q: %v\n", reg.Primary, err)
		os.Exit(1)
	}

	secondaryCount := len(reg.Accounts) - 1
	if secondaryCount <= 0 {
		fmt.Fprintln(os.Stderr, "cx watch: no secondary accounts configured; run: cx init <name>")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "cx watch: monitoring %q → %d secondary account(s) (Ctrl+C to stop)\n",
		reg.Primary, secondaryCount)

	// Build initial state snapshot.
	state := snapshotFiles(primaryDir)

	// Handle Ctrl+C / SIGTERM for clean shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(watchPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-quit:
			fmt.Fprintln(os.Stderr, "\ncx watch: stopped")
			return

		case <-ticker.C:
			changed := detectChanges(primaryDir, state)
			if len(changed) == 0 {
				continue
			}

			// Re-load registry in case accounts changed since startup.
			reg, err = config.LoadOrCreateRegistry(regPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "cx watch: reloading registry: %v\n", err)
				continue
			}

			synced := 0
			for name, acc := range reg.Accounts {
				if name == reg.Primary {
					continue
				}
				targetDir := acc.ConfigDir
				if targetDir == "" {
					continue
				}
				if err := syncFiles(primaryDir, targetDir, true); err != nil {
					fmt.Fprintf(os.Stderr, "cx watch: syncing %q: %v\n", name, err)
					continue
				}
				synced++
			}

			ts := time.Now().Format("2006-01-02 15:04")
			for _, f := range changed {
				fmt.Printf("[%s] synced %s to %dx\n", ts, f, synced)
			}

			// Update state snapshot after a successful sync pass.
			state = snapshotFiles(primaryDir)
		}
	}
}

// snapshotFiles records the mtime and size of every file in syncFileList that
// exists under dir.
func snapshotFiles(dir string) watchState {
	state := make(watchState, len(syncFileList))
	for _, rel := range syncFileList {
		path := filepath.Join(dir, rel)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		state[rel] = watchFileMeta{
			mtime: info.ModTime(),
			size:  info.Size(),
		}
	}
	return state
}

// detectChanges compares current file metadata against the stored snapshot and
// returns the list of relative file names that changed.
func detectChanges(dir string, prev watchState) []string {
	var changed []string
	for _, rel := range syncFileList {
		path := filepath.Join(dir, rel)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		fileMeta, seen := prev[rel]
		if !seen || info.ModTime().After(fileMeta.mtime) || info.Size() != fileMeta.size {
			changed = append(changed, rel)
		}
	}
	return changed
}
