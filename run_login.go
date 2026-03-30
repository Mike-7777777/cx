package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Mike-7777777/cx/internal/config"
	cxerrors "github.com/Mike-7777777/cx/internal/errors"
)

// loginCmd implements Runner for the "login" subcommand.
type loginCmd struct{}

// Run authenticates a Claude Code account. If a name is provided, logs in to
// that account's config directory; otherwise logs in to the current account.
func (c *loginCmd) Run(_ context.Context, app *App, args []string) error {
	var name string
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			printLoginHelp(app)
			return nil
		}
		if !strings.HasPrefix(arg, "-") && name == "" {
			name = arg
		}
	}

	// Determine config dir for the target account.
	var configDir string
	if name != "" {
		if !isValidAccountName(name) {
			return fmt.Errorf("invalid account name %q", name)
		}

		regPath, err := config.RegistryPath()
		if err != nil {
			return err
		}
		reg, err := config.LoadOrCreateRegistry(regPath)
		if err != nil {
			return err
		}
		dir, err := reg.ResolveConfigDir(name)
		if err != nil {
			if errors.Is(err, cxerrors.ErrAccountNotFound) {
				return fmt.Errorf("account %q not found (run 'cx config add %s' first)", name, name)
			}
			return err
		}
		configDir = dir
	} else {
		// Use current CLAUDE_CONFIG_DIR or detect main.
		configDir = os.Getenv("CLAUDE_CONFIG_DIR")
		if configDir == "" {
			d, err := config.DetectConfigDir()
			if err != nil {
				return err
			}
			configDir = d
		}
	}

	label := name
	if label == "" {
		label = "current"
	}
	fmt.Fprintf(app.Stderr, "[cx] Logging in to %s account (config: %s)...\n", label, configDir)

	if err := launchLogin(configDir); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	fmt.Fprintf(app.Stderr, "[cx] Login successful.\n")
	return nil
}

// launchLogin runs "claude auth login --claudeai" with CLAUDE_CONFIG_DIR set.
// The --claudeai flag skips the interactive "Select login method" menu.
// CC will attempt silent token refresh first; only opens browser if needed.
func launchLogin(configDir string) error {
	cmd := exec.Command("claude", "auth", "login", "--claudeai")
	cmd.Env = replaceOrAppendEnv(os.Environ(), "CLAUDE_CONFIG_DIR", configDir)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func printLoginHelp(app *App) {
	fmt.Fprint(app.Stderr, `cx login — authenticate a Claude Code account

Usage:
  cx login [name]

If <name> is provided, logs in to that account's config directory.
If omitted, logs in to the current account (based on CLAUDE_CONFIG_DIR).

Examples:
  cx login 5x       # log in to the 5x account
  cx login           # log in to the current account
  cx login 5x        # same, via shell wrapper
`)
}
