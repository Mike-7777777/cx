package main

import (
	"context"
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
	ok     bool
	warn   bool
	label  string
	detail string
	indent int
}

// doctorCmd implements Runner for the "doctor" subcommand.
type doctorCmd struct{}

// Run performs health checks on all registered accounts.
func (c *doctorCmd) Run(_ context.Context, app *App, args []string) error {
	useColor := app.UseColor
	var results []checkResult

	// Check main config dir.
	mainDir, err := config.DetectConfigDir()
	if err != nil {
		results = append(results, checkResult{
			label: "Main config", detail: err.Error(),
		})
		printResults(app, results, useColor)
		return fmt.Errorf("main config: %v", err)
	}
	results = append(results, checkResult{
		ok: true, label: "Main config", detail: mainDir,
	})

	// Check main credentials.
	mainCredOk, credDetail := checkCredentialsForDoctor(mainDir)
	results = append(results, checkResult{
		ok: mainCredOk, label: "Main credentials", detail: credDetail,
	})

	// Load registry.
	reg := app.Registry
	regPath := ""
	if rp, rpErr := config.RegistryPath(); rpErr == nil {
		regPath = rp
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

		// Check junction/symlink for shared dirs; auto-fix missing or plain-copy ones.
		for _, rel := range sharedLinkDirs {
			linkPath := filepath.Join(accDir, rel)
			target := filepath.Join(mainDir, rel)
			linkOk, linkMsg := checkLink(linkPath, target)

			needsFix := !linkOk || linkMsg == "exists (copy)"
			if needsFix {
				// Auto-fix: replace missing dir or plain copy with a proper junction/symlink.
				if _, statErr := os.Stat(target); statErr == nil {
					_ = os.RemoveAll(linkPath)
					if linkErr := platform.CreateLink(linkPath, target); linkErr == nil {
						linkOk = true
						linkMsg = "auto-fixed"
					}
				}
			}
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

		// Check settings.json sync with main (skip for main itself).
		if name != reg.Main {
			syncOk, syncMsg := checkSettingsSync(mainDir, accDir)
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
	// These override global config and are a common "statusline not working" cause.
	cwd, cwdErr := os.Getwd()
	if cwdErr == nil {
		projDir := filepath.Join(cwd, ".claude")
		if _, statErr := os.Stat(filepath.Join(projDir, "settings.json")); statErr == nil {
			pOk, pMsg := checkStatuslinePath(projDir)
			results = append(results, checkResult{
				ok: pOk, warn: !pOk, label: "project statusline",
				detail: pMsg,
			})
		}
	}

	printResults(app, results, useColor)
	return nil
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

	isSymlink := info.Mode()&os.ModeSymlink != 0

	// On Windows, junctions appear as regular directories to Go's os.Lstat.
	// Detect them by checking if the resolved path differs from the link path.
	isJunction := false
	if info.IsDir() && !isSymlink {
		resolved, evalErr := filepath.EvalSymlinks(linkPath)
		if evalErr == nil {
			isJunction = normalizePath(resolved) != normalizePath(linkPath)
		}
	}

	// Verify the link target is accessible.
	if _, err := os.Stat(linkPath); err != nil {
		return false, "broken link"
	}

	if isSymlink || isJunction {
		return true, "valid"
	}
	return true, "exists (copy)"
}

// checkSettingsSync compares settings.json between main and account dirs.
func checkSettingsSync(mainDir, accDir string) (bool, string) {
	mainPath := filepath.Join(mainDir, "settings.json")
	accPath := filepath.Join(accDir, "settings.json")

	mainData, err := os.ReadFile(mainPath)
	if os.IsNotExist(err) {
		return true, "no main settings.json"
	}
	if err != nil {
		return false, fmt.Sprintf("read main error: %v", err)
	}

	accData, err := os.ReadFile(accPath)
	if os.IsNotExist(err) {
		return false, "missing in account (run sync)"
	}
	if err != nil {
		return false, fmt.Sprintf("read error: %v", err)
	}

	if string(mainData) == string(accData) {
		return true, "in sync"
	}
	return false, "differs from main (run sync)"
}

// checkStatuslinePath verifies the statusLine command in settings.json is valid:
// - No backslash paths (breaks on some shells)
// - Binary at the path actually exists
// - Auto-fixes stale paths when possible
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

	needsWrite := false

	// Fix 1: backslash paths.
	if strings.Contains(cmd, "\\") {
		cmd = strings.ReplaceAll(cmd, "\\", "/")
		slMap["command"] = cmd
		needsWrite = true
	}

	// Fix 2: verify the binary exists (skip bare commands like "cx statusline").
	if !strings.HasPrefix(cmd, "cx ") {
		// Extract binary path from command (first space-separated token, unquoted).
		binPath := cmd
		if idx := strings.Index(cmd, " statusline"); idx > 0 {
			binPath = strings.Trim(cmd[:idx], "\"'")
		}
		if _, statErr := os.Stat(binPath); os.IsNotExist(statErr) {
			// Binary not found — try to auto-fix with current cx binary.
			if self, exeErr := os.Executable(); exeErr == nil {
				fixed := resolveStatuslineCommand(self)
				slMap["command"] = fixed
				needsWrite = true
				cmd = fixed
			} else {
				return false, fmt.Sprintf("binary not found: %s", binPath)
			}
		}
	}

	if needsWrite {
		settings["statusLine"] = slMap
		out, mErr := json.MarshalIndent(settings, "", "  ")
		if mErr == nil {
			if wErr := os.WriteFile(settingsPath, out, 0o600); wErr == nil {
				return true, fmt.Sprintf("auto-fixed → %s", cmd)
			}
		}
	}

	return true, "ok"
}

func printResults(app *App, results []checkResult, useColor bool) {
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
			fmt.Fprintf(app.Stdout, "%s%s %s: %s\n", indent, prefix, label, r.detail)
		} else {
			fmt.Fprintf(app.Stdout, "%s%s %s\n", indent, prefix, label)
		}
	}
}
