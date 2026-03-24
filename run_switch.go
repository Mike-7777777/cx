package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/MasaYan24/cc-monitor/internal/config"
	"github.com/MasaYan24/cc-monitor/internal/platform"
)

func runSwitch() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: cc-monitor switch <name> [--shell=bash|fish|powershell] [--no-sync]")
		os.Exit(1)
	}

	name := os.Args[2]
	noSync := hasFlag("--no-sync")
	shell := detectShellOverride()

	regPath, err := config.RegistryPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cc-monitor switch: %v\n", err)
		os.Exit(1)
	}

	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cc-monitor switch: %v\n", err)
		os.Exit(1)
	}

	configDir, err := reg.ResolveConfigDir(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cc-monitor switch: %v\n", err)
		os.Exit(1)
	}

	// Auto-sync non-primary accounts unless suppressed.
	isPrimary := name == reg.Primary
	if !noSync && !isPrimary {
		primaryDir, err := reg.ResolveConfigDir(reg.Primary)
		if err == nil {
			_ = syncFiles(primaryDir, configDir)
		}
	}

	// Emit shell commands to stdout — these are eval'd by the shell wrapper.
	switch shell {
	case platform.ShellFish:
		if isPrimary {
			fmt.Println("set -e CLAUDE_CONFIG_DIR")
		} else {
			fmt.Printf("set -gx CLAUDE_CONFIG_DIR %q\n", configDir)
		}
		fmt.Printf("echo '[cc-monitor] Switched to %s'\n", name)
	case platform.ShellPowerShell:
		if isPrimary {
			fmt.Println("Remove-Item Env:CLAUDE_CONFIG_DIR -ErrorAction SilentlyContinue")
		} else {
			fmt.Printf("$env:CLAUDE_CONFIG_DIR=%q\n", configDir)
		}
		fmt.Printf("Write-Host '[cc-monitor] Switched to %s'\n", name)
	default: // bash / zsh
		if isPrimary {
			fmt.Println("unset CLAUDE_CONFIG_DIR")
		} else {
			fmt.Printf("export CLAUDE_CONFIG_DIR=%q\n", configDir)
		}
		fmt.Printf("echo '[cc-monitor] Switched to %s'\n", name)
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
