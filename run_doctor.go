package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Mike-7777777/cx/internal/cache"
	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/format"
	"github.com/Mike-7777777/cx/internal/platform"
)

const (
	rateCacheStaleThreshold = 1 * time.Hour
)

type checkResult struct {
	ok      bool
	warn    bool
	label   string
	detail  string
	indent  int
}

func runDoctor() {
	useColor := platform.ANSIEnabled()
	var results []checkResult

	// Check primary config dir.
	primaryDir, err := config.DetectConfigDir()
	if err != nil {
		results = append(results, checkResult{
			label: "Primary config", detail: err.Error(),
		})
		printResults(results, useColor)
		os.Exit(1)
	}
	results = append(results, checkResult{
		ok: true, label: "Primary config", detail: primaryDir,
	})

	// Check primary credentials.
	primaryCredOk, credDetail := checkCredentialsForDoctor(primaryDir)
	results = append(results, checkResult{
		ok: primaryCredOk, label: "Primary credentials", detail: credDetail,
	})

	// Load registry.
	regPath, err := config.RegistryPath()
	if err != nil {
		results = append(results, checkResult{
			label: "Registry", detail: err.Error(),
		})
		printResults(results, useColor)
		os.Exit(1)
	}

	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		results = append(results, checkResult{
			label: "Registry", detail: err.Error(),
		})
		printResults(results, useColor)
		os.Exit(1)
	}

	results = append(results, checkResult{
		ok: true, label: "Registry", detail: regPath,
	})

	// Sort account names for deterministic order.
	names := make([]string, 0, len(reg.Accounts))
	for name := range reg.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)

	// Check each account.
	for _, name := range names {
		accDir, err := reg.ResolveConfigDir(name)
		if err != nil {
			results = append(results, checkResult{
				label: fmt.Sprintf("Account %s", name), detail: err.Error(),
			})
			continue
		}

		// Check config dir exists.
		if info, err := os.Stat(accDir); err != nil || !info.IsDir() {
			results = append(results, checkResult{
				label: fmt.Sprintf("Account %s", name), detail: fmt.Sprintf("config dir missing: %s", accDir),
			})
			continue
		}
		results = append(results, checkResult{
			ok: true, label: fmt.Sprintf("Account %s", name), detail: accDir,
		})

		// Check credentials.
		credOk, credMsg := checkCredentialsForDoctor(accDir)
		results = append(results, checkResult{
			ok: credOk, label: "credentials", detail: credMsg, indent: 1,
		})

		// Check junction/symlink for shared dirs.
		for _, rel := range sharedLinkDirs {
			linkPath := filepath.Join(accDir, rel)
			linkOk, linkMsg := checkLink(linkPath, filepath.Join(primaryDir, rel))
			results = append(results, checkResult{
				ok: linkOk, label: fmt.Sprintf("%s junction", rel), detail: linkMsg, indent: 1,
			})
		}

		// Check rate-cache freshness.
		rateCachePath := filepath.Join(accDir, "rate-cache.json")
		rc, rcErr := cache.ReadRateCache(rateCachePath)
		if rcErr != nil || rc == nil {
			results = append(results, checkResult{
				label: "rate-cache", detail: "not found", indent: 1,
			})
		} else {
			age := rc.Age()
			if age > rateCacheStaleThreshold {
				results = append(results, checkResult{
					warn: true, label: "rate-cache",
					detail: fmt.Sprintf("stale (%s ago)", format.FormatDuration(age)),
					indent: 1,
				})
			} else {
				results = append(results, checkResult{
					ok: true, label: "rate-cache",
					detail: fmt.Sprintf("fresh (%s ago)", format.FormatDuration(age)),
					indent: 1,
				})
			}
		}

		// Check settings.json sync with primary (skip for primary itself).
		if name != reg.Primary {
			syncOk, syncMsg := checkSettingsSync(primaryDir, accDir)
			results = append(results, checkResult{
				ok: syncOk, warn: !syncOk, label: "settings.json", detail: syncMsg, indent: 1,
			})
		}

		// Check statusline path in this account's settings.json.
		slOk, slMsg := checkStatuslinePath(accDir)
		results = append(results, checkResult{
			ok: slOk, warn: !slOk, label: "statusline path", detail: slMsg, indent: 1,
		})
	}

	// Check for project-level statusLine overrides in the current directory.
	// A .claude/settings.json with a statusLine key overrides the global config,
	// which is a common source of "statusline not working" issues.
	cwd, cwdErr := os.Getwd()
	if cwdErr == nil {
		projSettingsPath := filepath.Join(cwd, ".claude", "settings.json")
		if projData, readErr := os.ReadFile(projSettingsPath); readErr == nil {
			var projSettings map[string]any
			if json.Unmarshal(projData, &projSettings) == nil {
				if sl, hasSL := projSettings["statusLine"]; hasSL {
					slMap, _ := sl.(map[string]any)
					cmd, _ := slMap["command"].(string)
					if cmd != "" && !strings.Contains(cmd, "cx") {
						results = append(results, checkResult{
							warn: true, label: "project statusline override",
							detail: fmt.Sprintf(".claude/settings.json overrides global statusline: %s", cmd),
						})
					} else if strings.Contains(cmd, "cc-monitor") {
						results = append(results, checkResult{
							label: "project statusline override",
							detail: ".claude/settings.json points to old cc-monitor path",
						})
					} else {
						results = append(results, checkResult{
							ok: true, label: "project statusline",
							detail: "ok",
						})
					}
				}
			}
		}
	}

	printResults(results, useColor)
}

// checkCredentialsForDoctor verifies .credentials.json exists and has claudeAiOauth.
// Returns (ok, detail) for the doctor report output.
func checkCredentialsForDoctor(dir string) (bool, string) {
	status := checkCredentials(dir)
	switch status {
	case credentialOK:
		return true, "valid"
	case credentialMissing:
		return false, "missing or unreadable"
	case credentialNoToken:
		return false, "no access token"
	case credentialExpired:
		return false, "expired (no refresh token)"
	default:
		return false, "unknown"
	}
}

// checkLink verifies that linkPath is a valid junction/symlink pointing to target.
func checkLink(linkPath, target string) (bool, string) {
	info, err := os.Lstat(linkPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Check if target exists; if not, the link doesn't need to exist.
			if _, tErr := os.Stat(target); os.IsNotExist(tErr) {
				return true, "skipped (target absent)"
			}
			return false, "missing"
		}
		return false, fmt.Sprintf("error: %v", err)
	}

	// On Windows, junctions appear as directories with the reparse point attribute.
	// On Unix, check for symlink mode bit.
	isLink := info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeDir != 0

	// Verify the link target is accessible.
	if _, err := os.Stat(linkPath); err != nil {
		return false, "broken link"
	}

	if isLink {
		return true, "valid"
	}
	return true, "exists (copy)"
}

// checkSettingsSync compares settings.json between primary and account dirs.
func checkSettingsSync(primaryDir, accDir string) (bool, string) {
	primaryPath := filepath.Join(primaryDir, "settings.json")
	accPath := filepath.Join(accDir, "settings.json")

	primaryData, err := os.ReadFile(primaryPath)
	if os.IsNotExist(err) {
		return true, "no primary settings.json"
	}
	if err != nil {
		return false, fmt.Sprintf("read primary error: %v", err)
	}

	accData, err := os.ReadFile(accPath)
	if os.IsNotExist(err) {
		return false, "missing in account (run sync)"
	}
	if err != nil {
		return false, fmt.Sprintf("read error: %v", err)
	}

	if string(primaryData) == string(accData) {
		return true, "in sync"
	}
	return false, "differs from primary (run sync)"
}

// checkStatuslinePath verifies the statusLine command in settings.json uses
// forward slashes. Backslash paths break Git Bash stdin piping on Windows.
func checkStatuslinePath(configDir string) (bool, string) {
	settingsPath := filepath.Join(configDir, "settings.json")
	data, err := os.ReadFile(settingsPath)
	if os.IsNotExist(err) {
		return true, "no settings.json"
	}
	if err != nil {
		return false, fmt.Sprintf("read error: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return false, fmt.Sprintf("parse error: %v", err)
	}

	sl, ok := settings["statusLine"]
	if !ok {
		return true, "not configured"
	}

	slMap, ok := sl.(map[string]any)
	if !ok {
		return false, "invalid format"
	}

	cmd, ok := slMap["command"].(string)
	if !ok {
		return false, "no command field"
	}

	if strings.Contains(cmd, "\\") {
		// Auto-fix: rewrite with forward slashes.
		fixed := strings.ReplaceAll(cmd, "\\", "/")
		slMap["command"] = fixed
		settings["statusLine"] = slMap
		out, mErr := json.MarshalIndent(settings, "", "  ")
		if mErr == nil {
			if wErr := os.WriteFile(settingsPath, out, 0o600); wErr == nil {
				return true, fmt.Sprintf("auto-fixed backslash path → %s", fixed)
			}
		}
		return false, fmt.Sprintf("backslash in path breaks Git Bash piping: %s", cmd)
	}

	if strings.Contains(cmd, "cx") || strings.Contains(cmd, "cc-monitor") {
		return true, "ok"
	}
	return true, fmt.Sprintf("custom: %s", cmd)
}

func printResults(results []checkResult, useColor bool) {
	for _, r := range results {
		var prefix string
		indent := ""
		if r.indent > 0 {
			indent = "  "
		}

		switch {
		case r.ok:
			prefix = format.Colorize("[ok]", format.Green, useColor)
		case r.warn:
			prefix = format.Colorize("[warn]", format.Yellow, useColor)
		default:
			prefix = format.Colorize("[FAIL]", format.Red, useColor)
		}

		label := r.label
		if r.detail != "" {
			fmt.Printf("%s%s %s: %s\n", indent, prefix, label, r.detail)
		} else {
			fmt.Printf("%s%s %s\n", indent, prefix, label)
		}
	}
}
