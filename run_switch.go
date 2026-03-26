package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	cxerrors "github.com/Mike-7777777/cx/internal/errors"
	"github.com/Mike-7777777/cx/internal/platform"
)

// safePathPattern rejects shell metacharacters in paths that will be eval'd.
// Allows letters, digits, slashes, backslashes, dots, hyphens, underscores,
// colons (Windows drives), spaces, and tildes.
var safePathPattern = regexp.MustCompile(`^[a-zA-Z0-9/\\.:\-_ ~]+$`)

// switchCmd implements Runner for the "switch" subcommand.
type switchCmd struct {
	shell  platform.Shell
	noSync bool
}

// Run validates the account, optionally syncs config files, checks credentials,
// and writes shell commands to app.Stdout for eval by the shell wrapper.
func (c *switchCmd) Run(_ context.Context, app *App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: cx switch <name> [--shell=bash|fish|powershell] [--no-sync]")
	}

	name := args[0]
	if !validAccountName.MatchString(name) {
		return fmt.Errorf("invalid account name %q (only letters, digits, hyphens, underscores)", name)
	}

	configDir, err := app.Registry.ResolveConfigDir(name)
	if err != nil {
		if errors.Is(err, cxerrors.ErrAccountNotFound) {
			return fmt.Errorf("account %q not found (run 'cx init %s' first)", name, name)
		}
		return err
	}

	// Sanitize configDir before interpolating into eval'd shell commands.
	// This prevents command injection via a tampered registry file.
	configDir = filepath.Clean(configDir)
	if !filepath.IsAbs(configDir) || !safePathPattern.MatchString(configDir) {
		return cxerrors.ErrUnsafeConfigPath
	}

	// Auto-sync non-main accounts unless suppressed.
	isMain := name == app.Registry.Main
	if !c.noSync && !isMain {
		mainDir, err := app.Registry.ResolveConfigDir(app.Registry.Main)
		if err == nil {
			_ = syncFiles(mainDir, configDir, true)
		}
	}

	// Check if credentials need attention (local check only — no network calls).
	needsLogin := checkCredentials(configDir) != credentialOK

	// Emit shell commands to stdout — these are eval'd by the shell wrapper.
	switch c.shell {
	case platform.ShellFish:
		if isMain {
			fmt.Fprintln(app.Stdout, "set -e CLAUDE_CONFIG_DIR")
		} else {
			fmt.Fprintf(app.Stdout, "set -gx CLAUDE_CONFIG_DIR \"%s\"\n", configDir)
		}
		fmt.Fprintf(app.Stdout, "echo '[cx] Switched to %s'\n", name)
	case platform.ShellPowerShell:
		if isMain {
			fmt.Fprintln(app.Stdout, "Remove-Item Env:CLAUDE_CONFIG_DIR -ErrorAction SilentlyContinue")
		} else {
			fmt.Fprintf(app.Stdout, "$env:CLAUDE_CONFIG_DIR=\"%s\"\n", configDir)
		}
		fmt.Fprintf(app.Stdout, "Write-Host '[cx] Switched to %s'\n", name)
	default: // bash / zsh
		if isMain {
			fmt.Fprintln(app.Stdout, "unset CLAUDE_CONFIG_DIR")
		} else {
			fmt.Fprintf(app.Stdout, "export CLAUDE_CONFIG_DIR=\"%s\"\n", configDir)
		}
		fmt.Fprintf(app.Stdout, "echo '[cx] Switched to %s'\n", name)
	}

	// If credentials are missing/invalid, append login to the eval output.
	// Uses --claudeai to skip the interactive "Select login method" menu.
	// CC will attempt silent token refresh first; only opens browser if needed.
	if needsLogin {
		switch c.shell {
		case platform.ShellPowerShell:
			fmt.Fprintln(app.Stdout, "Write-Host '[cx] Credentials need refresh — logging in...' -ForegroundColor Yellow")
			fmt.Fprintln(app.Stdout, "& claude auth login --claudeai")
		default: // bash / zsh / fish
			fmt.Fprintln(app.Stdout, "echo '[cx] Credentials need refresh — logging in...'")
			fmt.Fprintln(app.Stdout, "claude auth login --claudeai")
		}
	}

	return nil
}

// runSwitch is the legacy entry point dispatched by main.go.
func runSwitch() {
	args := os.Args[2:]
	flags, positional := parseFlags(args, "shell", "no-sync")

	shell := platform.DetectShell()
	if val, ok := flags["shell"]; ok {
		switch strings.ToLower(val) {
		case "fish":
			shell = platform.ShellFish
		case "powershell":
			shell = platform.ShellPowerShell
		default:
			shell = platform.ShellBash
		}
	}

	_, noSync := flags["no-sync"]

	cmd := &switchCmd{shell: shell, noSync: noSync}

	app, err := buildApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cx switch: %v\n", err)
		os.Exit(1)
	}

	if err := cmd.Run(context.Background(), app, positional); err != nil {
		fmt.Fprintf(os.Stderr, "cx switch: %v\n", err)
		os.Exit(1)
	}
}
