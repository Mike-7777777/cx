package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	cxerrors "github.com/Mike-7777777/cx/internal/errors"
)

// Account represents a single named Claude Code account in the registry.
type Account struct {
	ConfigDir string `json:"config_dir"`
	Email     string `json:"email,omitempty"`
	Alias     string `json:"alias,omitempty"`
}

// StatuslineConfig controls which sections are shown in the statusline.
// All fields default to true when nil (pointer types for "not set" = default).
type StatuslineConfig struct {
	ShowAccount      *bool `json:"show_account,omitempty"`
	ShowCost         *bool `json:"show_cost,omitempty"`
	ShowContext      *bool `json:"show_context,omitempty"`
	ShowRate5h       *bool `json:"show_rate_5h,omitempty"`
	ShowRate7d       *bool `json:"show_rate_7d,omitempty"`
	ShowOtherAccount *bool `json:"show_other_account,omitempty"`
	ShowSwitchHint   *bool `json:"show_switch_hint,omitempty"`
}

// IsEnabled returns the value of a *bool field, defaulting to true when nil.
func (c *StatuslineConfig) IsEnabled(field *bool) bool {
	if field == nil {
		return true
	}
	return *field
}

// Registry holds all known accounts and tracks which is primary.
// The path field is intentionally unexported so it is never serialised.
type Registry struct {
	Version    int                `json:"version"`
	Primary    string             `json:"primary"`
	Accounts   map[string]Account `json:"accounts"`
	Statusline *StatuslineConfig  `json:"statusline,omitempty"`
	path       string
}

// LoadOrCreateRegistry loads the registry from path, or returns a new empty
// registry if the file does not yet exist.
func LoadOrCreateRegistry(path string) (*Registry, error) {
	r := &Registry{
		Version:  1,
		Accounts: make(map[string]Account),
		path:     path,
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading registry %q: %w", path, err)
	}

	if err := json.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("parsing registry %q: %w", path, err)
	}

	// Ensure the map is always non-nil after unmarshalling.
	if r.Accounts == nil {
		r.Accounts = make(map[string]Account)
	}
	r.path = path
	return r, nil
}

// AddAccount inserts or updates the account identified by name.
// If the account already exists, only ConfigDir is updated; metadata
// fields (Email, Alias) are preserved.
func (r *Registry) AddAccount(name, configDir string) {
	if existing, ok := r.Accounts[name]; ok {
		existing.ConfigDir = configDir
		r.Accounts[name] = existing
	} else {
		r.Accounts[name] = Account{ConfigDir: configDir}
	}
}

// ResolveConfigDir returns the config directory for the named account.
// When the stored config_dir is empty, it falls back to DefaultConfigDir
// (ignoring CLAUDE_CONFIG_DIR) because an empty config_dir means "use the
// default location", not "follow the current session's override".
// Returns ErrAccountNotFound when the account does not exist.
func (r *Registry) ResolveConfigDir(name string) (string, error) {
	acc, ok := r.Accounts[name]
	if !ok {
		return "", fmt.Errorf("account %q: %w", name, cxerrors.ErrAccountNotFound)
	}
	if acc.ConfigDir != "" {
		return acc.ConfigDir, nil
	}
	return DefaultConfigDir()
}

// Save writes the registry to its path with mode 0600.
func (r *Registry) Save() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o700); err != nil {
		return fmt.Errorf("creating registry directory: %w", err)
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("serialising registry: %w", err)
	}

	if err := os.WriteFile(r.path, data, 0o600); err != nil {
		return fmt.Errorf("writing registry %q: %w", r.path, err)
	}
	return nil
}

// RegistryPath returns the default registry file location (~/.cx.json).
func RegistryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".cx.json"), nil
}
