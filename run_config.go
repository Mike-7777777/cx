package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/format"
)

// configCmd implements Runner for the "config" subcommand.
type configCmd struct{}

// Run dispatches to config subcommands: show (default), main, rename, set.
func (c *configCmd) Run(_ context.Context, app *App, args []string) error {
	if len(args) == 0 || args[0] == "show" {
		return configShow(app)
	}

	switch args[0] {
	case "main":
		if len(args) < 2 {
			return fmt.Errorf("usage: cx config main <account-name>")
		}
		return configSetMain(app, args[1])
	case "rename":
		if len(args) < 3 {
			return fmt.Errorf("usage: cx config rename <old-name> <new-name>")
		}
		return configRename(app, args[1], args[2])
	case "set":
		if len(args) < 4 {
			return fmt.Errorf("usage: cx config set <account> <key> <value>\n  keys: email, alias")
		}
		return configSet(app, args[1], args[2], strings.Join(args[3:], " "))
	case "--help", "-h", "help":
		configHelp(app)
		return nil
	default:
		configHelp(app)
		return fmt.Errorf("unknown config subcommand: %s", args[0])
	}
}

func configShow(app *App) error {
	reg := app.Registry
	useColor := app.UseColor

	fmt.Fprintln(app.Stdout, format.Colorize("cx configuration", format.Bold, useColor))
	fmt.Fprintln(app.Stdout)

	names := sortedAccountNames(reg)

	for _, name := range names {
		acc := reg.Accounts[name]
		dir := resolveDir(reg, name)

		// Main marker.
		marker := "  "
		if name == reg.Main {
			marker = format.Colorize("★ ", format.Yellow, useColor)
		}

		// Single read for tier + identity.
		info := readAccountInfo(dir)

		nameStr := format.Colorize(name, format.Cyan+format.Bold, useColor)
		fmt.Fprintf(app.Stdout, "%s%s", marker, nameStr)
		if info.Tier != "" {
			fmt.Fprintf(app.Stdout, " (%s)", format.Colorize(info.Tier, format.Green, useColor))
		}
		fmt.Fprintln(app.Stdout)

		if info.Email != "" {
			fmt.Fprintf(app.Stdout, "    email: %s\n", info.Email)
		} else if acc.Email != "" {
			fmt.Fprintf(app.Stdout, "    email: %s\n", acc.Email)
		}
		if info.DisplayName != "" {
			fmt.Fprintf(app.Stdout, "    user:  %s\n", info.DisplayName)
		}
		if acc.Alias != "" {
			fmt.Fprintf(app.Stdout, "    alias: %s\n", acc.Alias)
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
			fmt.Fprintf(app.Stdout, "    stats: %s\n", format.Colorize(strings.Join(meta, " · "), format.Dim, useColor))
		}

		// Config dir.
		dirLabel := dir
		if acc.ConfigDir == "" {
			dirLabel += " (default)"
		}
		fmt.Fprintf(app.Stdout, "    dir:   %s\n", format.Colorize(dirLabel, format.Dim, useColor))
		fmt.Fprintln(app.Stdout)
	}
	return nil
}

func configSetMain(app *App, name string) error {
	// Reload from disk since we modify and save.
	reg, err := reloadRegistry()
	if err != nil {
		return err
	}

	if _, ok := reg.Accounts[name]; !ok {
		return fmt.Errorf("account %q not found", name)
	}

	old := reg.Main
	reg.Main = name
	if err := reg.Save(); err != nil {
		return fmt.Errorf("saving registry: %v", err)
	}

	fmt.Fprintf(app.Stderr, "Main changed: %s → %s\n", old, name)
	return nil
}

func configRename(app *App, oldName, newName string) error {
	if !validAccountName.MatchString(newName) {
		return fmt.Errorf("invalid name %q (only letters, digits, hyphens, underscores)", newName)
	}

	// Reload from disk since we modify and save.
	reg, err := reloadRegistry()
	if err != nil {
		return err
	}

	acc, ok := reg.Accounts[oldName]
	if !ok {
		return fmt.Errorf("account %q not found", oldName)
	}

	if _, exists := reg.Accounts[newName]; exists {
		return fmt.Errorf("account %q already exists", newName)
	}

	// Move the account entry.
	delete(reg.Accounts, oldName)
	reg.Accounts[newName] = acc

	// Update main if it was the renamed account.
	if reg.Main == oldName {
		reg.Main = newName
	}

	if err := reg.Save(); err != nil {
		return fmt.Errorf("saving registry: %v", err)
	}

	fmt.Fprintf(app.Stderr, "Renamed: %s → %s\n", oldName, newName)

	// Hint: directory is not renamed (by design).
	if acc.ConfigDir != "" {
		fmt.Fprintf(app.Stderr, "  Note: config directory unchanged at %s\n", acc.ConfigDir)
	}
	return nil
}

func configSet(app *App, accountName, key, value string) error {
	// Reload from disk since we modify and save.
	reg, err := reloadRegistry()
	if err != nil {
		return err
	}

	acc, ok := reg.Accounts[accountName]
	if !ok {
		return fmt.Errorf("account %q not found", accountName)
	}

	switch key {
	case "email":
		acc.Email = value
	case "alias":
		acc.Alias = value
	default:
		return fmt.Errorf("unknown key %q (supported: email, alias)", key)
	}

	reg.Accounts[accountName] = acc
	if err := reg.Save(); err != nil {
		return fmt.Errorf("saving registry: %v", err)
	}

	fmt.Fprintf(app.Stderr, "Set %s.%s = %s\n", accountName, key, value)
	return nil
}

func configHelp(app *App) {
	fmt.Fprint(app.Stderr, `cx config — manage accounts and settings

Usage:
  cx config                        Show full configuration
  cx config main <name>            Change main account
  cx config rename <old> <new>     Rename an account
  cx config set <name> email <v>   Set account email
  cx config set <name> alias <v>   Set account alias
`)
}

// reloadRegistry loads a fresh registry from disk for commands that modify and save it.
func reloadRegistry() (*config.Registry, error) {
	regPath, err := config.RegistryPath()
	if err != nil {
		return nil, err
	}
	return config.LoadOrCreateRegistry(regPath)
}

// resolveDir resolves the config directory for an account.
func resolveDir(reg *config.Registry, name string) string {
	dir, err := reg.ResolveConfigDir(name)
	if err != nil {
		return "?"
	}
	return filepath.Clean(dir)
}

// sortedAccountNames returns account names sorted, main first.
func sortedAccountNames(reg *config.Registry) []string {
	names := make([]string, 0, len(reg.Accounts))
	for name := range reg.Accounts {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		// Main account always comes first.
		if names[i] == reg.Main {
			return true
		}
		if names[j] == reg.Main {
			return false
		}
		return names[i] < names[j]
	})
	return names
}
