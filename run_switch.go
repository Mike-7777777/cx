package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Mike-7777777/cx/internal/config"
	cxerrors "github.com/Mike-7777777/cx/internal/errors"
	"github.com/Mike-7777777/cx/internal/platform"
)

// safePathPattern rejects shell metacharacters in paths that will be eval'd.
// Allows letters, digits, slashes, backslashes, dots, hyphens, underscores,
// colons (Windows drives), spaces, and tildes.
var safePathPattern = regexp.MustCompile(`^[a-zA-Z0-9/\\.:\-_ ~]+$`)

func runSwitch() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: cx switch <name> [--shell=bash|fish|powershell] [--no-sync]")
		os.Exit(1)
	}

	name := os.Args[2]
	if !validAccountName.MatchString(name) {
		fmt.Fprintf(os.Stderr, "cx switch: invalid account name %q (only letters, digits, hyphens, underscores)\n", name)
		os.Exit(1)
	}
	noSync := hasFlag("--no-sync")
	shell := detectShellOverride()

	regPath, err := config.RegistryPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cx switch: %v\n", err)
		os.Exit(1)
	}

	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cx switch: %v\n", err)
		os.Exit(1)
	}

	configDir, err := reg.ResolveConfigDir(name)
	if err != nil {
		if errors.Is(err, cxerrors.ErrAccountNotFound) {
			fmt.Fprintf(os.Stderr, "cx switch: account %q not found (run 'cx init %s' first)\n", name, name)
		} else {
			fmt.Fprintf(os.Stderr, "cx switch: %v\n", err)
		}
		os.Exit(1)
	}

	// Sanitize configDir before interpolating into eval'd shell commands.
	// This prevents command injection via a tampered registry file.
	configDir = filepath.Clean(configDir)
	if !filepath.IsAbs(configDir) || !safePathPattern.MatchString(configDir) {
		fmt.Fprintf(os.Stderr, "cx switch: %v\n", cxerrors.ErrUnsafeConfigPath)
		os.Exit(1)
	}

	// Auto-sync non-primary accounts unless suppressed.
	isPrimary := name == reg.Primary
	if !noSync && !isPrimary {
		primaryDir, err := reg.ResolveConfigDir(reg.Primary)
		if err == nil {
			_ = syncFiles(primaryDir, configDir, true)
		}
	}

	// Check if credentials need attention (local check only — no network calls).
	needsLogin := checkCredentials(configDir) != credentialOK

	// Emit shell commands to stdout — these are eval'd by the shell wrapper.
	switch shell {
	case platform.ShellFish:
		if isPrimary {
			fmt.Println("set -e CLAUDE_CONFIG_DIR")
		} else {
			fmt.Printf("set -gx CLAUDE_CONFIG_DIR \"%s\"\n", configDir)
		}
		fmt.Printf("echo '[cx] Switched to %s'\n", name)
	case platform.ShellPowerShell:
		if isPrimary {
			fmt.Println("Remove-Item Env:CLAUDE_CONFIG_DIR -ErrorAction SilentlyContinue")
		} else {
			fmt.Printf("$env:CLAUDE_CONFIG_DIR=\"%s\"\n", configDir)
		}
		fmt.Printf("Write-Host '[cx] Switched to %s'\n", name)
	default: // bash / zsh
		if isPrimary {
			fmt.Println("unset CLAUDE_CONFIG_DIR")
		} else {
			fmt.Printf("export CLAUDE_CONFIG_DIR=\"%s\"\n", configDir)
		}
		fmt.Printf("echo '[cx] Switched to %s'\n", name)
	}

	// If credentials are missing/invalid, append login to the eval output.
	// Uses --claudeai to skip the interactive "Select login method" menu.
	// CC will attempt silent token refresh first; only opens browser if needed.
	if needsLogin {
		switch shell {
		case platform.ShellPowerShell:
			fmt.Println("Write-Host '[cx] Credentials need refresh — logging in...' -ForegroundColor Yellow")
			fmt.Println("& claude auth login --claudeai")
		default: // bash / zsh / fish
			fmt.Println("echo '[cx] Credentials need refresh — logging in...'")
			fmt.Println("claude auth login --claudeai")
		}
	}
}

// detectShellOverride reads the --shell=<type> flag from os.Args and returns
// the matching Shell, falling back to platform.DetectShell if absent.
func detectShellOverride() platform.Shell {
	for _, arg := range os.Args[3:] {
		if strings.HasPrefix(arg, "--shell=") {
			val := strings.TrimPrefix(arg, "--shell=")
			switch strings.ToLower(val) {
			case "fish":
				return platform.ShellFish
			case "powershell":
				return platform.ShellPowerShell
			default:
				return platform.ShellBash
			}
		}
	}
	return platform.DetectShell()
}
