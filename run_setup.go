package main

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/platform"
)

// setupCmd implements Runner for the "setup" subcommand.
type setupCmd struct{}

//go:embed skill/cx.md
var skillFS embed.FS

// Run performs interactive first-time setup: account registration, shell
// wrapper installation, statusline configuration, and health check.
func (c *setupCmd) Run(_ context.Context, app *App, _ []string) error {
	w := app.Stderr
	reader := bufio.NewReader(os.Stdin)
	fmt.Fprintln(w, "[cx] Interactive setup")
	fmt.Fprintln(w)

	// Step 1: Detect and register main account.
	mainDir, err := config.DetectConfigDir()
	if err != nil {
		fmt.Fprintf(w, "[cx] Cannot detect main Claude Code config: %v\n", err)
		fmt.Fprintln(w, "  Make sure Claude Code is installed and you've logged in at least once.")
		return fmt.Errorf("cannot detect main config: %w", err)
	}

	regPath, err := config.RegistryPath()
	if err != nil {
		return fmt.Errorf("registry path: %w", err)
	}

	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	// Ask for main account name.
	defaultMain := reg.Main
	if defaultMain == "" {
		defaultMain = "main"
	}
	fmt.Fprintf(w, "  Main account detected: %s\n", mainDir)
	fmt.Fprintf(w, "  Name for main account [%s]: ", defaultMain)
	mainName := readLine(reader)
	if mainName == "" {
		mainName = defaultMain
	}
	if !validAccountName.MatchString(mainName) {
		return fmt.Errorf("invalid name %q", mainName)
	}

	reg.Main = mainName
	reg.AddAccount(mainName, "")
	if err := reg.Save(); err != nil {
		return fmt.Errorf("saving registry: %w", err)
	}
	fmt.Fprintf(w, "  Registered main account %q\n\n", mainName)

	// Step 2: Create secondary accounts.
	fmt.Fprint(w, "  How many additional accounts? [0]: ")
	countStr := readLine(reader)
	count := 0
	if countStr != "" {
		for _, c := range countStr {
			if c < '0' || c > '9' {
				return fmt.Errorf("invalid number %q", countStr)
			}
			count = count*10 + int(c-'0')
		}
	}

	home, _ := os.UserHomeDir()

	for i := 0; i < count; i++ {
		fmt.Fprintf(w, "\n  Account %d name: ", i+1)
		name := readLine(reader)
		if name == "" {
			fmt.Fprintln(w, "  skipped (empty name)")
			continue
		}
		if !validAccountName.MatchString(name) {
			fmt.Fprintf(w, "  invalid name %q, skipping\n", name)
			continue
		}

		targetDir := filepath.Join(home, ".claude-"+name)

		// Check if already exists.
		if _, statErr := os.Stat(targetDir); statErr == nil {
			if _, exists := reg.Accounts[name]; exists {
				fmt.Fprintf(w, "  %q already exists, skipping init\n", name)
			} else {
				// Directory exists but not registered.
				reg.AddAccount(name, targetDir)
				if err := reg.Save(); err != nil {
					fmt.Fprintf(w, "  warning: saving registry: %v\n", err)
				}
				fmt.Fprintf(w, "  %q already exists, registered\n", name)
			}
		} else {
			// Create new account directory.
			fmt.Fprintf(w, "  Creating %s...\n", targetDir)
			if err := os.MkdirAll(targetDir, 0o700); err != nil {
				fmt.Fprintf(w, "  failed to create dir: %v\n", err)
				continue
			}

			// Create junctions/symlinks.
			for _, rel := range sharedLinkDirs {
				src := filepath.Join(mainDir, rel)
				if _, statErr := os.Stat(src); os.IsNotExist(statErr) {
					continue
				}
				dst := filepath.Join(targetDir, rel)
				_ = os.MkdirAll(filepath.Dir(dst), 0o700)
				_ = os.RemoveAll(dst)
				if linkErr := platform.CreateLink(dst, src); linkErr != nil {
					fmt.Fprintf(w, "  warning: linking %s: %v\n", rel, linkErr)
				} else {
					fmt.Fprintf(w, "  linked %s\n", rel)
				}
			}

			// Sync config.
			if err := syncFiles(mainDir, targetDir, true); err != nil {
				fmt.Fprintf(w, "  warning: syncing config: %v\n", err)
			}

			// Register.
			reg.AddAccount(name, targetDir)
			if err := reg.Save(); err != nil {
				fmt.Fprintf(w, "  warning: saving registry: %v\n", err)
			}
			fmt.Fprintf(w, "  Registered %q\n", name)
		}

		// Check credentials; login if needed.
		status := checkCredentials(targetDir)
		if status != credentialOK {
			fmt.Fprintf(w, "  Launching login for %q...\n", name)
			if loginErr := launchLogin(targetDir); loginErr != nil {
				fmt.Fprintf(w, "  Login failed: %v (retry later with: cc login %s)\n", loginErr, name)
			} else {
				fmt.Fprintf(w, "  Login successful for %q\n", name)
			}
		} else {
			fmt.Fprintf(w, "  %q already has valid credentials\n", name)
		}
	}

	// Step 3: Install shell wrapper for all relevant shells.
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Install 'cx' shell wrapper:")

	type shellOption struct {
		shell  platform.Shell
		name   string
		exists bool // profile file exists
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
		fmt.Fprintf(w, "    Install for %s? [Y/n]: ", opt.name)
		answer := readLine(reader)
		if answer == "" || strings.ToLower(answer) == "y" {
			if installed := installShellWrapper(opt.shell); installed {
				fmt.Fprintf(w, "    Installed for %s.\n", opt.name)
			}
		} else {
			fmt.Fprintf(w, "    Skipped %s.\n", opt.name)
		}
	}
	fmt.Fprintln(w, "  Restart your shell(s) to activate the wrapper.")

	// Step 4: Ensure cx is in PATH (for non-interactive shells like CC's bash).
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Add cx to PATH (for Claude Code and non-interactive shells)? [Y/n]: ")
	pathAnswer := readLine(reader)
	if pathAnswer == "" || strings.ToLower(pathAnswer) == "y" {
		ensureInPath()
	} else {
		fmt.Fprintln(w, "  Skipped PATH installation.")
	}

	// Step 5: Configure CC statusline.
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Configure Claude Code statusline? [Y/n]: ")
	slAnswer := readLine(reader)
	if slAnswer == "" || strings.ToLower(slAnswer) == "y" {
		if configureStatusline(mainDir) {
			fmt.Fprintln(w, "  Statusline configured. cx will show rate limits in CC's status bar.")
		}
	} else {
		fmt.Fprintln(w, "  Skipped statusline configuration.")
	}

	// Step 6: Install Claude Code skill.
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Install Claude Code skill (enables /cx inside CC)? [Y/n]: ")
	skillAnswer := readLine(reader)
	if skillAnswer == "" || strings.ToLower(skillAnswer) == "y" {
		if installSkill(mainDir) {
			fmt.Fprintln(w, "  Skill installed. Use /cx inside Claude Code to check status and usage.")
		}
	} else {
		fmt.Fprintln(w, "  Skipped skill installation.")
	}

	// Step 7: Health check.
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Running health check...")
	fmt.Fprintln(w)
	// Build a temporary App to run the migrated doctor command.
	if doctorApp, doctorErr := buildApp(); doctorErr == nil {
		_ = (&doctorCmd{}).Run(context.Background(), doctorApp, nil)
	}

	// Step 8: Show usage.
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Setup complete! Quick reference:")
	fmt.Fprintln(w)

	// List registered accounts.
	reg, _ = config.LoadOrCreateRegistry(regPath)
	for name := range reg.Accounts {
		if name == reg.Main {
			fmt.Fprintf(w, "    cx switch %s     — switch to main\n", name)
		} else {
			fmt.Fprintf(w, "    cx switch %s     — switch to %s\n", name, name)
		}
	}
	fmt.Fprintln(w, "    cx run       — auto-select best account")
	fmt.Fprintln(w, "    cx status     — see all accounts")
	fmt.Fprintln(w, "    cx dashboard  — live TUI dashboard")
	fmt.Fprintln(w)
	return nil
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
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString("\n" + wrapper + "\n"); err != nil {
		fmt.Fprintf(os.Stderr, "  Failed to write to %s: %v\n", profilePath, err)
		return false
	}

	fmt.Fprintf(os.Stderr, "  Added cx() wrapper to %s\n", profilePath)
	return true
}

// findWrapperEnd locates the end of the cx wrapper block starting at idx.
// It searches for the closing marker (} or end) at column 0 — i.e., preceded
// by a newline. Inner braces are always indented and won't match.
func findWrapperEnd(content string, idx int, shell platform.Shell) int {
	var endMarker string
	switch shell {
	case platform.ShellFish:
		endMarker = "end"
	default: // bash, zsh, powershell all end with "}"
		endMarker = "}"
	}

	// Find the last endMarker at line start (column 0) within the block.
	// Check for endMarker at file start (no preceding newline).
	needle := "\n" + endMarker
	pos := idx
	lastEnd := idx

	if strings.HasPrefix(content[idx:], endMarker) {
		lastEnd = idx + len(endMarker)
	}
	for {
		next := strings.Index(content[pos:], needle)
		if next < 0 {
			break
		}
		candidate := pos + next + len(needle)
		lastEnd = candidate
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

// resolveStatuslineCommand returns the best statusline command string.
// If cx is in PATH, returns the portable "cx statusline".
// Otherwise returns the absolute path with forward slashes.
func resolveStatuslineCommand(cxPath string) string {
	// Check if "cx" resolves to the same binary via PATH.
	if found, err := exec.LookPath("cx"); err == nil {
		foundAbs, _ := filepath.Abs(found)
		selfAbs, _ := filepath.Abs(cxPath)
		if normalizePath(foundAbs) == normalizePath(selfAbs) {
			return "cx statusline"
		}
	}
	return strings.ReplaceAll(cxPath, "\\", "/") + " statusline"
}

// configureStatusline adds the cx statusline command to CC's settings.json.
// It reads the existing settings, merges the statusLine config, and writes
// back atomically. Returns true if configuration succeeded.
func configureStatusline(mainConfigDir string) bool {
	settingsPath := filepath.Join(mainConfigDir, "settings.json")

	// Find the cx binary path.
	cxPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Could not determine cx binary path: %v\n", err)
		return false
	}

	// Read existing settings.
	var settings map[string]any
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		settings = make(map[string]any)
	} else {
		if err := json.Unmarshal(data, &settings); err != nil {
			fmt.Fprintf(os.Stderr, "  Could not parse settings.json: %v\n", err)
			return false
		}
	}

	// Set statusLine config.
	// Prefer bare "cx statusline" if cx is in PATH (portable across installs).
	// Otherwise fall back to absolute path with forward slashes.
	cmd := resolveStatuslineCommand(cxPath)
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

// ensureInPath creates wrapper scripts in a PATH directory so cx is callable
// from any shell context, including non-interactive shells like Claude Code's
// bash. On Windows, creates both a sh wrapper (for Git Bash / CC) and a .cmd
// wrapper (for cmd.exe). On Unix, creates a sh wrapper.
func ensureInPath() {
	cxPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Could not determine cx binary path: %v\n", err)
		return
	}

	// Already directly in PATH as a binary (not a wrapper)?
	if found, lookErr := exec.LookPath("cx"); lookErr == nil {
		foundAbs, _ := filepath.Abs(found)
		selfAbs, _ := filepath.Abs(cxPath)
		if normalizePath(foundAbs) == normalizePath(selfAbs) {
			fmt.Fprintln(os.Stderr, "  cx binary is already in PATH.")
			return
		}
	}

	home, _ := os.UserHomeDir()
	binDir := findBinDir(home)

	if err := os.MkdirAll(binDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "  Could not create %s: %v\n", binDir, err)
		return
	}

	cxAbsPath := strings.ReplaceAll(cxPath, "\\", "/")

	// sh wrapper (Git Bash, CC's bash, Unix shells).
	shPath := filepath.Join(binDir, "cx")
	shContent := "#!/bin/sh\n# Generated by cx setup\nexec \"" + cxAbsPath + "\" \"$@\"\n"
	if err := os.WriteFile(shPath, []byte(shContent), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "  Could not write %s: %v\n", shPath, err)
		return
	}
	fmt.Fprintf(os.Stderr, "  Installed %s\n", shPath)

	// Windows: also create cx.cmd for cmd.exe.
	if runtime.GOOS == "windows" {
		cmdPath := filepath.Join(binDir, "cx.cmd")
		winPath := strings.ReplaceAll(cxPath, "/", "\\")
		cmdContent := "@rem Generated by cx setup\r\n@\"" + winPath + "\" %*\r\n"
		if err := os.WriteFile(cmdPath, []byte(cmdContent), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: could not write %s: %v\n", cmdPath, err)
		} else {
			fmt.Fprintf(os.Stderr, "  Installed %s\n", cmdPath)
		}
	}
}

// findBinDir returns a user-writable directory in PATH suitable for wrapper
// scripts. Checks ~/bin and ~/.local/bin; returns the first one found in PATH.
// If neither is in PATH, defaults to ~/bin with a warning.
func findBinDir(home string) string {
	candidates := []string{
		filepath.Join(home, "bin"),
		filepath.Join(home, ".local", "bin"),
	}

	pathDirs := filepath.SplitList(os.Getenv("PATH"))
	for _, c := range candidates {
		cAbs, _ := filepath.Abs(c)
		for _, p := range pathDirs {
			pAbs, _ := filepath.Abs(p)
			if normalizePath(cAbs) == normalizePath(pAbs) {
				return c
			}
		}
	}

	// Neither found in PATH — default to ~/bin and warn.
	fmt.Fprintf(os.Stderr, "  Note: ~/bin is not in PATH. Add it for cx to work everywhere.\n")
	return candidates[0]
}

// installSkill writes the embedded cx skill file to the skills/ subdirectory
// of the main Claude Code config dir, enabling /cx inside CC conversations.
func installSkill(mainConfigDir string) bool {
	content, err := skillFS.ReadFile("skill/cx.md")
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Could not read embedded skill file: %v\n", err)
		return false
	}

	skillDir := filepath.Join(mainConfigDir, "skills")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "  Could not create skills directory: %v\n", err)
		return false
	}

	skillPath := filepath.Join(skillDir, "cx.md")
	if err := os.WriteFile(skillPath, content, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "  Could not write skill file: %v\n", err)
		return false
	}

	fmt.Fprintf(os.Stderr, "  Installed %s\n", skillPath)
	return true
}
