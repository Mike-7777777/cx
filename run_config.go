package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/format"
	"github.com/Mike-7777777/cx/internal/platform"
)

func runConfig() {
	args := os.Args[2:] // everything after "config"

	if len(args) == 0 || args[0] == "show" {
		configShow()
		return
	}

	switch args[0] {
	case "primary":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: cx config primary <account-name>")
			os.Exit(1)
		}
		configSetPrimary(args[1])
	case "rename":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: cx config rename <old-name> <new-name>")
			os.Exit(1)
		}
		configRename(args[1], args[2])
	case "set":
		if len(args) < 4 {
			fmt.Fprintln(os.Stderr, "usage: cx config set <account> <key> <value>")
			fmt.Fprintln(os.Stderr, "  keys: email, alias")
			os.Exit(1)
		}
		configSet(args[1], args[2], strings.Join(args[3:], " "))
	case "--help", "-h", "help":
		configHelp()
	default:
		fmt.Fprintf(os.Stderr, "unknown config subcommand: %s\n", args[0])
		configHelp()
		os.Exit(1)
	}
}

func configShow() {
	reg := loadRegistry()
	useColor := platform.ANSIEnabled()

	fmt.Println(format.Colorize("cx configuration", format.Bold, useColor))
	fmt.Println()

	names := sortedAccountNames(reg)

	for _, name := range names {
		acc := reg.Accounts[name]
		dir := resolveDir(reg, name)

		// Primary marker.
		marker := "  "
		if name == reg.Primary {
			marker = format.Colorize("★ ", format.Yellow, useColor)
		}

		// Single read for tier + identity.
		info := readAccountInfo(dir)

		nameStr := format.Colorize(name, format.Cyan+format.Bold, useColor)
		fmt.Printf("%s%s", marker, nameStr)
		if info.Tier != "" {
			fmt.Printf(" (%s)", format.Colorize(info.Tier, format.Green, useColor))
		}
		fmt.Println()

		if info.Email != "" {
			fmt.Printf("    email: %s\n", info.Email)
		} else if acc.Email != "" {
			fmt.Printf("    email: %s\n", acc.Email)
		}
		if info.DisplayName != "" {
			fmt.Printf("    user:  %s\n", info.DisplayName)
		}
		if acc.Alias != "" {
			fmt.Printf("    alias: %s\n", acc.Alias)
		}

		// Stats from .claude.json.
		var meta []string
		if info.CCVersion != "" {
			meta = append(meta, fmt.Sprintf("CC %s", info.CCVersion))
		}
		if info.NumStartups > 0 {
			meta = append(meta, fmt.Sprintf("%d sessions", info.NumStartups))
		}
		if len(meta) > 0 {
			fmt.Printf("    stats: %s\n", format.Colorize(strings.Join(meta, " · "), format.Dim, useColor))
		}

		// Config dir.
		dirLabel := dir
		if acc.ConfigDir == "" {
			dirLabel += " (default)"
		}
		fmt.Printf("    dir:   %s\n", format.Colorize(dirLabel, format.Dim, useColor))
		fmt.Println()
	}
}

func configSetPrimary(name string) {
	reg := loadRegistry()

	if _, ok := reg.Accounts[name]; !ok {
		fmt.Fprintf(os.Stderr, "account %q not found\n", name)
		os.Exit(1)
	}

	old := reg.Primary
	reg.Primary = name
	if err := reg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "saving registry: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Primary changed: %s → %s\n", old, name)
}

func configRename(oldName, newName string) {
	reg := loadRegistry()

	if !validAccountName.MatchString(newName) {
		fmt.Fprintf(os.Stderr, "invalid name %q (only letters, digits, hyphens, underscores)\n", newName)
		os.Exit(1)
	}

	acc, ok := reg.Accounts[oldName]
	if !ok {
		fmt.Fprintf(os.Stderr, "account %q not found\n", oldName)
		os.Exit(1)
	}

	if _, exists := reg.Accounts[newName]; exists {
		fmt.Fprintf(os.Stderr, "account %q already exists\n", newName)
		os.Exit(1)
	}

	// Move the account entry.
	delete(reg.Accounts, oldName)
	reg.Accounts[newName] = acc

	// Update primary if it was the renamed account.
	if reg.Primary == oldName {
		reg.Primary = newName
	}

	if err := reg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "saving registry: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Renamed: %s → %s\n", oldName, newName)

	// Hint: directory is not renamed (by design).
	if acc.ConfigDir != "" {
		fmt.Fprintf(os.Stderr, "  Note: config directory unchanged at %s\n", acc.ConfigDir)
	}
}

func configSet(accountName, key, value string) {
	reg := loadRegistry()

	acc, ok := reg.Accounts[accountName]
	if !ok {
		fmt.Fprintf(os.Stderr, "account %q not found\n", accountName)
		os.Exit(1)
	}

	switch key {
	case "email":
		acc.Email = value
	case "alias":
		acc.Alias = value
	default:
		fmt.Fprintf(os.Stderr, "unknown key %q (supported: email, alias)\n", key)
		os.Exit(1)
	}

	reg.Accounts[accountName] = acc
	if err := reg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "saving registry: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Set %s.%s = %s\n", accountName, key, value)
}

func configHelp() {
	fmt.Print(`cx config — manage accounts and settings

Usage:
  cx config                        Show full configuration
  cx config primary <name>         Change primary account
  cx config rename <old> <new>     Rename an account
  cx config set <name> email <v>   Set account email
  cx config set <name> alias <v>   Set account alias
`)
}

// loadRegistry loads the registry or exits on error.
func loadRegistry() *config.Registry {
	regPath, err := config.RegistryPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cx config: %v\n", err)
		os.Exit(1)
	}
	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cx config: %v\n", err)
		os.Exit(1)
	}
	return reg
}

// resolveDir resolves the config directory for an account.
func resolveDir(reg *config.Registry, name string) string {
	dir, err := reg.ResolveConfigDir(name)
	if err != nil {
		return "?"
	}
	return filepath.Clean(dir)
}

// sortedAccountNames returns account names sorted, primary first.
func sortedAccountNames(reg *config.Registry) []string {
	names := make([]string, 0, len(reg.Accounts))
	for name := range reg.Accounts {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		// Primary account always comes first.
		if names[i] == reg.Primary {
			return true
		}
		if names[j] == reg.Primary {
			return false
		}
		return names[i] < names[j]
	})
	return names
}
