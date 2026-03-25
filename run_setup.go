package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/platform"
)

func runSetup() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Fprintln(os.Stderr, "[cx] Interactive setup")
	fmt.Fprintln(os.Stderr)

	// Step 1: Detect and register primary account.
	primaryDir, err := config.DetectConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[cx] Cannot detect primary Claude Code config: %v\n", err)
		fmt.Fprintln(os.Stderr, "  Make sure Claude Code is installed and you've logged in at least once.")
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

	// Ask for primary account name.
	defaultPrimary := reg.Primary
	if defaultPrimary == "" {
		defaultPrimary = "primary"
	}
	fmt.Fprintf(os.Stderr, "  Primary account detected: %s\n", primaryDir)
	fmt.Fprintf(os.Stderr, "  Name for primary account [%s]: ", defaultPrimary)
	primaryName := readLine(reader)
	if primaryName == "" {
		primaryName = defaultPrimary
	}
	if !validAccountName.MatchString(primaryName) {
		fmt.Fprintf(os.Stderr, "[cx] invalid name %q\n", primaryName)
		os.Exit(1)
	}

	reg.Primary = primaryName
	reg.AddAccount(primaryName, "")
	if err := reg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "[cx] saving registry: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "  Registered primary account %q\n\n", primaryName)

	// Step 2: Create secondary accounts.
	fmt.Fprint(os.Stderr, "  How many additional accounts? [0]: ")
	countStr := readLine(reader)
	count := 0
	if countStr != "" {
		for _, c := range countStr {
			if c < '0' || c > '9' {
				fmt.Fprintln(os.Stderr, "[cx] invalid number")
				os.Exit(1)
			}
			count = count*10 + int(c-'0')
		}
	}

	home, _ := os.UserHomeDir()

	for i := 0; i < count; i++ {
		fmt.Fprintf(os.Stderr, "\n  Account %d name: ", i+1)
		name := readLine(reader)
		if name == "" {
			fmt.Fprintln(os.Stderr, "  skipped (empty name)")
			continue
		}
		if !validAccountName.MatchString(name) {
			fmt.Fprintf(os.Stderr, "  invalid name %q, skipping\n", name)
			continue
		}

		targetDir := filepath.Join(home, ".claude-"+name)

		// Check if already exists.
		if _, statErr := os.Stat(targetDir); statErr == nil {
			if _, exists := reg.Accounts[name]; exists {
				fmt.Fprintf(os.Stderr, "  %q already exists, skipping init\n", name)
			} else {
				// Directory exists but not registered.
				reg.AddAccount(name, targetDir)
				if err := reg.Save(); err != nil {
					fmt.Fprintf(os.Stderr, "  warning: saving registry: %v\n", err)
				}
				fmt.Fprintf(os.Stderr, "  %q already exists, registered\n", name)
			}
		} else {
			// Create new account directory.
			fmt.Fprintf(os.Stderr, "  Creating %s...\n", targetDir)
			if err := os.MkdirAll(targetDir, 0o700); err != nil {
				fmt.Fprintf(os.Stderr, "  failed to create dir: %v\n", err)
				continue
			}

			// Create junctions/symlinks.
			for _, rel := range sharedLinkDirs {
				src := filepath.Join(primaryDir, rel)
				if _, statErr := os.Stat(src); os.IsNotExist(statErr) {
					continue
				}
				dst := filepath.Join(targetDir, rel)
				_ = os.MkdirAll(filepath.Dir(dst), 0o700)
				_ = os.RemoveAll(dst)
				if linkErr := platform.CreateLink(dst, src); linkErr != nil {
					fmt.Fprintf(os.Stderr, "  warning: linking %s: %v\n", rel, linkErr)
				} else {
					fmt.Fprintf(os.Stderr, "  linked %s\n", rel)
				}
			}

			// Sync config.
			if err := syncFiles(primaryDir, targetDir, true); err != nil {
				fmt.Fprintf(os.Stderr, "  warning: syncing config: %v\n", err)
			}

			// Register.
			reg.AddAccount(name, targetDir)
			if err := reg.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "  warning: saving registry: %v\n", err)
			}
			fmt.Fprintf(os.Stderr, "  Registered %q\n", name)
		}

		// Check credentials; login if needed.
		status := checkCredentials(targetDir)
		if status != credentialOK {
			fmt.Fprintf(os.Stderr, "  Launching login for %q...\n", name)
			if loginErr := launchLogin(targetDir); loginErr != nil {
				fmt.Fprintf(os.Stderr, "  Login failed: %v (retry later with: cc login %s)\n", loginErr, name)
			} else {
				fmt.Fprintf(os.Stderr, "  Login successful for %q\n", name)
			}
		} else {
			fmt.Fprintf(os.Stderr, "  %q already has valid credentials\n", name)
		}
	}

	// Step 3: Install shell wrapper for all relevant shells.
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  Install 'cx' shell wrapper:")

	type shellOption struct {
		shell   platform.Shell
		name    string
		exists  bool // profile file exists
	}

	options := []shellOption{
		{platform.ShellPowerShell, "PowerShell", false},
		{platform.ShellBash, "Bash/Zsh", false},
		{platform.ShellFish, "Fish", false},
	}

	// Check which shells have profile files.
	for i, opt := range options {
		profilePath, _ := shellWrapperConfig(opt.shell)
		if profilePath != "" {
			// For bash/zsh/fish, check if profile exists or if the shell binary exists.
			if _, err := os.Stat(profilePath); err == nil {
				options[i].exists = true
			}
		}
	}

	// PowerShell always available on Windows.
	if runtime.GOOS == "windows" {
		options[0].exists = true
	}

	for _, opt := range options {
		if !opt.exists {
			continue
		}
		fmt.Fprintf(os.Stderr, "    Install for %s? [Y/n]: ", opt.name)
		answer := readLine(reader)
		if answer == "" || strings.ToLower(answer) == "y" {
			if installed := installShellWrapper(opt.shell); installed {
				fmt.Fprintf(os.Stderr, "    Installed for %s.\n", opt.name)
			}
		} else {
			fmt.Fprintf(os.Stderr, "    Skipped %s.\n", opt.name)
		}
	}
	fmt.Fprintln(os.Stderr, "  Restart your shell(s) to activate the wrapper.")

	// Step 4: Configure CC statusline.
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  Configure Claude Code statusline? [Y/n]: ")
	slAnswer := readLine(reader)
	if slAnswer == "" || strings.ToLower(slAnswer) == "y" {
		if configureStatusline(primaryDir) {
			fmt.Fprintln(os.Stderr, "  Statusline configured. cx will show rate limits in CC's status bar.")
		}
	} else {
		fmt.Fprintln(os.Stderr, "  Skipped statusline configuration.")
	}

	// Step 5: Health check.
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  Running health check...")
	fmt.Fprintln(os.Stderr)
	runDoctor()

	// Step 5: Show usage.
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  Setup complete! Quick reference:")
	fmt.Fprintln(os.Stderr)

	// List registered accounts.
	reg, _ = config.LoadOrCreateRegistry(regPath)
	for name := range reg.Accounts {
		if name == reg.Primary {
			fmt.Fprintf(os.Stderr, "    cx switch %s     — switch to primary\n", name)
		} else {
			fmt.Fprintf(os.Stderr, "    cx switch %s     — switch to %s\n", name, name)
		}
	}
	fmt.Fprintln(os.Stderr, "    cx run       — auto-select best account")
	fmt.Fprintln(os.Stderr, "    cx status     — see all accounts")
	fmt.Fprintln(os.Stderr, "    cx dashboard  — live TUI dashboard")
	fmt.Fprintln(os.Stderr)
}

// readLine reads a trimmed line from reader.
func readLine(reader *bufio.Reader) string {
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

// installShellWrapper writes or updates the cx() wrapper function in the
// appropriate shell profile file. If an older wrapper exists, it is replaced
// (e.g., when the cx binary moves to a new path). Returns true on success.
func installShellWrapper(shell platform.Shell) bool {
	profilePath, wrapper := shellWrapperConfig(shell)
	if profilePath == "" {
		fmt.Fprintln(os.Stderr, "  Could not determine profile path.")
		return false
	}

	existing, _ := os.ReadFile(profilePath)
	content := string(existing)

	// Detect and replace old wrapper (handles path changes / upgrades).
	if idx := strings.Index(content, "# cx: Claude Code multi-account"); idx >= 0 {
		// Find the end of the wrapper block.
		end := findWrapperEnd(content, idx, shell)
		oldWrapper := content[idx:end]
		if oldWrapper == wrapper {
			fmt.Fprintf(os.Stderr, "  Wrapper already up to date in %s\n", profilePath)
			return true
		}
		// Replace old wrapper with new one.
		content = content[:idx] + wrapper + "\n" + content[end:]
		if err := os.WriteFile(profilePath, []byte(content), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "  Failed to update %s: %v\n", profilePath, err)
			return false
		}
		fmt.Fprintf(os.Stderr, "  Updated cx() wrapper in %s\n", profilePath)
		return true
	}

	// No existing wrapper — append.
	f, err := os.OpenFile(profilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Failed to open %s: %v\n", profilePath, err)
		return false
	}
	defer f.Close()

	if _, err := f.WriteString("\n" + wrapper + "\n"); err != nil {
		fmt.Fprintf(os.Stderr, "  Failed to write to %s: %v\n", profilePath, err)
		return false
	}

	fmt.Fprintf(os.Stderr, "  Added cx() wrapper to %s\n", profilePath)
	return true
}

// findWrapperEnd locates the end of the cx wrapper block starting at idx.
func findWrapperEnd(content string, idx int, shell platform.Shell) int {
	// Each shell wrapper ends with a closing brace/keyword.
	var endMarker string
	switch shell {
	case platform.ShellFish:
		endMarker = "end"
	default: // bash, zsh, powershell all end with "}"
		endMarker = "}"
	}

	// Scan forward from idx to find the last closing marker of the block.
	pos := idx
	lastEnd := idx
	for {
		next := strings.Index(content[pos:], endMarker)
		if next < 0 {
			break
		}
		candidate := pos + next + len(endMarker)
		lastEnd = candidate
		// Stop at first blank line or EOF after the marker.
		if candidate >= len(content) || content[candidate] == '\n' || content[candidate] == '\r' {
			break
		}
		pos = candidate
	}

	// Skip trailing newlines.
	for lastEnd < len(content) && (content[lastEnd] == '\n' || content[lastEnd] == '\r') {
		lastEnd++
	}
	return lastEnd
}

// shellWrapperConfig returns the profile path and wrapper code for each shell.
// PowerShell embeds the absolute path to cx.exe because PowerShell has no
// equivalent of bash's `command` builtin to bypass function name resolution.
// Bash/Zsh/Fish use `command cx` which requires cx to be in PATH.
func shellWrapperConfig(shell platform.Shell) (string, string) {
	home, _ := os.UserHomeDir()

	// Resolve absolute path for PowerShell wrapper.
	cxPath, _ := os.Executable()
	cxPath = strings.ReplaceAll(cxPath, "\\", "/") // forward slashes for all shells

	switch shell {
	case platform.ShellPowerShell:
		profilePath := powershellProfilePath()
		wrapper := fmt.Sprintf(`# cx: Claude Code multi-account wrapper (path set by cx setup)
$cxExe = "%s"
function cx {
    if ($args[0] -eq "switch") {
        $cmd = & $cxExe switch $args[1] --shell=powershell
        Invoke-Expression ($cmd -join "`+"`"+`n")
    } else {
        & $cxExe @args
    }
}`, cxPath)
		return profilePath, wrapper

	case platform.ShellFish:
		profilePath := filepath.Join(home, ".config", "fish", "functions", "cx.fish")
		wrapper := `# cx: Claude Code multi-account wrapper
function cx
    if test "$argv[1]" = "switch"
        eval (command cx switch $argv[2])
    else
        command cx $argv
    end
end`
		return profilePath, wrapper

	default: // Bash / Zsh
		// Prefer .bashrc, fall back to .zshrc.
		profilePath := filepath.Join(home, ".bashrc")
		if _, err := os.Stat(profilePath); os.IsNotExist(err) {
			profilePath = filepath.Join(home, ".zshrc")
		}
		wrapper := `# cx: Claude Code multi-account wrapper
cx() {
    case "$1" in
        switch) eval "$(command cx switch "$2")" ;;
        *) command cx "$@" ;;
    esac
}`
		return profilePath, wrapper
	}
}

// powershellProfilePath returns the PowerShell profile path.
func powershellProfilePath() string {
	// On Windows, check common profile locations.
	if runtime.GOOS == "windows" {
		home, _ := os.UserHomeDir()
		candidates := []string{
			filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"),
			filepath.Join(home, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"),
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
		// Default to PowerShell 7 location, create parent if needed.
		profileDir := filepath.Join(home, "Documents", "PowerShell")
		_ = os.MkdirAll(profileDir, 0o755)
		return filepath.Join(profileDir, "Microsoft.PowerShell_profile.ps1")
	}
	// On Unix, PowerShell profile is in ~/.config/powershell/
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "powershell", "Microsoft.PowerShell_profile.ps1")
}

// configureStatusline adds the cx statusline command to CC's settings.json.
// It reads the existing settings, merges the statusLine config, and writes
// back atomically. Returns true if configuration succeeded.
func configureStatusline(primaryConfigDir string) bool {
	settingsPath := filepath.Join(primaryConfigDir, "settings.json")

	// Find the cx binary path.
	cxPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Could not determine cx binary path: %v\n", err)
		return false
	}

	// Read existing settings.
	var settings map[string]interface{}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		settings = make(map[string]interface{})
	} else {
		if err := json.Unmarshal(data, &settings); err != nil {
			fmt.Fprintf(os.Stderr, "  Could not parse settings.json: %v\n", err)
			return false
		}
	}

	// Set statusLine config.
	// CC runs statusline commands through Git Bash on Windows, so paths
	// must use forward slashes to avoid broken stdin pipes.
	cmd := strings.ReplaceAll(cxPath, "\\", "/") + " statusline"
	settings["statusLine"] = map[string]string{
		"type":    "command",
		"command": cmd,
	}

	// Write back.
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Could not marshal settings: %v\n", err)
		return false
	}

	if err := os.WriteFile(settingsPath, out, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "  Could not write settings.json: %v\n", err)
		return false
	}

	fmt.Fprintf(os.Stderr, "  Updated %s\n", settingsPath)
	return true
}
