package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Mike-7777777/cx/internal/config"
	cxerrors "github.com/Mike-7777777/cx/internal/errors"
)

func runLogin() {
	args := os.Args[2:]

	var name string
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			printLoginHelp()
			return
		}
		if !strings.HasPrefix(arg, "-") && name == "" {
			name = arg
		}
	}

	// Determine config dir for the target account.
	var configDir string
	if name != "" {
		if !validAccountName.MatchString(name) {
			fmt.Fprintf(os.Stderr, "[cx] invalid account name %q\n", name)
			os.Exit(1)
		}

		regPath, err := config.RegistryPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[cx] %v\n", err)
			os.Exit(1)
		}
		reg, err := config.LoadOrCreateRegistry(regPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[cx] %v\n", err)
			os.Exit(1)
		}
		dir, err := reg.ResolveConfigDir(name)
		if err != nil {
			if errors.Is(err, cxerrors.ErrAccountNotFound) {
				fmt.Fprintf(os.Stderr, "[cx] account %q not found (run 'cx init %s' first)\n", name, name)
			} else {
				fmt.Fprintf(os.Stderr, "[cx] %v\n", err)
			}
			os.Exit(1)
		}
		configDir = dir
	} else {
		// Use current CLAUDE_CONFIG_DIR or detect main.
		configDir = os.Getenv("CLAUDE_CONFIG_DIR")
		if configDir == "" {
			d, err := config.DetectConfigDir()
			if err != nil {
				fmt.Fprintf(os.Stderr, "[cx] %v\n", err)
				os.Exit(1)
			}
			configDir = d
		}
	}

	label := name
	if label == "" {
		label = "current"
	}
	fmt.Fprintf(os.Stderr, "[cx] Logging in to %s account (config: %s)...\n", label, configDir)

	if err := launchLogin(configDir); err != nil {
		fmt.Fprintf(os.Stderr, "[cx] login failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "[cx] Login successful.\n")
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

func printLoginHelp() {
	fmt.Print(`cx login — authenticate a Claude Code account

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
