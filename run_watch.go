package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Mike-7777777/cx/internal/config"
)

const (
	watchPollInterval = 30 * time.Second
)

// watchState tracks the last-seen mtime and size of each synced file in the
// main config directory so we can detect changes on each poll cycle.
type watchState map[string]watchFileMeta

type watchFileMeta struct {
	mtime time.Time
	size  int64
}

// watchCmd implements Runner for the "watch" subcommand.
type watchCmd struct{}

// Run polls the main account's config files at a fixed interval and
// auto-syncs to all secondaries when a change is detected.
// It exits gracefully when ctx is cancelled.
func (c *watchCmd) Run(ctx context.Context, app *App, args []string) error {
	regPath, err := config.RegistryPath()
	if err != nil {
		return err
	}

	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		return err
	}

	if reg.Main == "" {
		return fmt.Errorf("no main account configured; run: cx init <name>")
	}

	mainDir, err := reg.ResolveConfigDir(reg.Main)
	if err != nil {
		return fmt.Errorf("resolving main account %q: %v", reg.Main, err)
	}

	secondaryCount := len(reg.Accounts) - 1
	if secondaryCount <= 0 {
		return fmt.Errorf("no secondary accounts configured; run: cx init <name>")
	}

	fmt.Fprintf(app.Stderr, "cx watch: monitoring %q → %d secondary account(s) (Ctrl+C to stop)\n",
		reg.Main, secondaryCount)

	// Build initial state snapshot.
	state := snapshotFiles(mainDir)

	ticker := time.NewTicker(watchPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(app.Stderr, "\ncx watch: stopped")
			return nil

		case <-ticker.C:
			changed := detectChanges(mainDir, state)
			if len(changed) == 0 {
				continue
			}

			// Re-load registry in case accounts changed since startup.
			reg, err = config.LoadOrCreateRegistry(regPath)
			if err != nil {
				fmt.Fprintf(app.Stderr, "cx watch: reloading registry: %v\n", err)
				continue
			}

			synced := 0
			for name, acc := range reg.Accounts {
				if name == reg.Main {
					continue
				}
				targetDir := acc.ConfigDir
				if targetDir == "" {
					continue
				}
				if err := syncFiles(mainDir, targetDir, true); err != nil {
					fmt.Fprintf(app.Stderr, "cx watch: syncing %q: %v\n", name, err)
					continue
				}
				synced++
			}

			ts := time.Now().Format("2006-01-02 15:04")
			for _, f := range changed {
				fmt.Fprintf(app.Stderr, "[%s] synced %s to %dx\n", ts, f, synced)
			}

			// Update state snapshot after a successful sync pass.
			state = snapshotFiles(mainDir)
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
