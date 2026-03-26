package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/platform"
)

// validAccountName restricts account names to safe alphanumeric characters,
// hyphens, and underscores to prevent path traversal attacks.
var validAccountName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// sharedLinkDirs are subdirectories that are junctioned/symlinked from the
// main config dir into every secondary account dir.
var sharedLinkDirs = []string{
	filepath.Join("plugins", "cache"),
	"ide",
	"skills",
	"projects",
}

// initCmd implements Runner for the "init" subcommand.
type initCmd struct{}

// Run creates a new account directory, sets up shared links, syncs config
// files, registers the account, and launches login.
func (c *initCmd) Run(_ context.Context, app *App, args []string) error {
	flags, positional := parseFlags(args, "force")

	if len(positional) == 0 {
		return fmt.Errorf("usage: cx init <name> [--force]")
	}

	name := positional[0]
	if !validAccountName.MatchString(name) {
		return fmt.Errorf("invalid account name %q (only letters, digits, hyphens, underscores)", name)
	}
	_, force := flags["force"]

	// Validate: main config dir must exist.
	mainDir, err := config.DetectConfigDir()
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	targetDir := filepath.Join(home, ".claude-"+name)

	if _, err := os.Stat(targetDir); err == nil && !force {
		return fmt.Errorf("%q already exists; use --force to overwrite", targetDir)
	}

	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return fmt.Errorf("creating target dir: %v", err)
	}

	// Create junctions/symlinks for shared dirs.
	for _, rel := range sharedLinkDirs {
		src := filepath.Join(mainDir, rel)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}

		dst := filepath.Join(targetDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return fmt.Errorf("creating parent for link %q: %v", dst, err)
		}
		// Remove existing link/dir before recreating.
		_ = os.RemoveAll(dst)
		if err := platform.CreateLink(dst, src); err != nil {
			return fmt.Errorf("linking %q → %q: %v", dst, src, err)
		}
		fmt.Fprintf(app.Stderr, "  linked %s\n", rel)
	}

	// Sync shared config files.
	if err := syncFiles(mainDir, targetDir, true); err != nil {
		return fmt.Errorf("syncing files: %v", err)
	}

	// Register account in registry.
	regPath, err := config.RegistryPath()
	if err != nil {
		return err
	}

	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		return err
	}

	// If no main account is registered yet, detect and set it.
	if reg.Main == "" {
		reg.Main = "main"
		reg.AddAccount("main", "")
	}

	reg.AddAccount(name, targetDir)

	if err := reg.Save(); err != nil {
		return fmt.Errorf("saving registry: %v", err)
	}

	fmt.Fprintf(app.Stderr, "initialized account %q at %s\n", name, targetDir)

	// Auto-launch login for the new account.
	fmt.Fprintf(app.Stderr, "[cx] Launching login for %q...\n", name)
	if err := launchLogin(targetDir); err != nil {
		fmt.Fprintf(app.Stderr, "[cx] login failed: %v\n", err)
		fmt.Fprintf(app.Stderr, "  retry later with: cx login %s\n", name)
	} else {
		fmt.Fprintf(app.Stderr, "[cx] Login successful for %q.\n", name)
	}

	return nil
}

// hasFlagFrom reports whether flag appears verbatim in os.Args[start:].
func hasFlagFrom(flag string, start int) bool {
	if len(os.Args) <= start {
		return false
	}
	for _, arg := range os.Args[start:] {
		if arg == flag {
			return true
		}
	}
	return false
}
