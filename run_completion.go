package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/Mike-7777777/cx/internal/config"
)

// completionCmd implements Runner for the "completion" subcommand.
type completionCmd struct{}

// Run outputs a shell completion script to stdout for the given shell type.
func (c *completionCmd) Run(_ context.Context, app *App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: cx completion <bash|fish|powershell>")
	}

	reg := app.Registry
	shell := strings.ToLower(args[0])
	switch shell {
	case "bash":
		fmt.Fprint(app.Stdout, bashCompletion(reg))
	case "fish":
		fmt.Fprint(app.Stdout, fishCompletion(reg))
	case "powershell":
		fmt.Fprint(app.Stdout, powershellCompletion(reg))
	default:
		return fmt.Errorf("unsupported shell: %s (supported: bash, fish, powershell)", shell)
	}
	return nil
}

func bashCompletion(reg *config.Registry) string {
	accounts := sortedAccountNames(reg)
	accountList := strings.Join(accounts, " ")

	return fmt.Sprintf(`# bash completion for cx
# Add to ~/.bashrc: eval "$(cx completion bash)"

_cx_completions() {
    local cur prev commands
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    commands="completion config daemon dashboard doctor insights login predict resume run sessions setup status statusline switch sync usage web version help"

    case "${prev}" in
        cx)
            COMPREPLY=( $(compgen -W "${commands}" -- "${cur}") )
            return 0
            ;;
        switch)
            COMPREPLY=( $(compgen -W "%s" -- "${cur}") )
            return 0
            ;;
        usage)
            COMPREPLY=( $(compgen -W "daily weekly monthly session blocks messages" -- "${cur}") )
            return 0
            ;;
        completion)
            COMPREPLY=( $(compgen -W "bash fish powershell" -- "${cur}") )
            return 0
            ;;
        run)
            COMPREPLY=( $(compgen -W "--prefer --balance --help" -- "${cur}") )
            return 0
            ;;
        --prefer)
            COMPREPLY=( $(compgen -W "%s" -- "${cur}") )
            return 0
            ;;
    esac
}
complete -F _cx_completions cx
`, accountList, accountList)
}

func fishCompletion(reg *config.Registry) string {
	accounts := sortedAccountNames(reg)

	var sb strings.Builder
	sb.WriteString(`# fish completion for cx
# Add to ~/.config/fish/completions/cx.fish

# Disable file completions by default
complete -c cx -f

# Subcommands
complete -c cx -n '__fish_use_subcommand' -a 'setup' -d 'One-time interactive setup'
complete -c cx -n '__fish_use_subcommand' -a 'switch' -d 'Switch active Claude Code account'
complete -c cx -n '__fish_use_subcommand' -a 'run' -d 'Auto-select best account and launch claude'
complete -c cx -n '__fish_use_subcommand' -a 'config' -d 'Manage accounts and metadata'
complete -c cx -n '__fish_use_subcommand' -a 'sessions' -d 'List recent CC sessions'
complete -c cx -n '__fish_use_subcommand' -a 'resume' -d 'Resume a CC session'
complete -c cx -n '__fish_use_subcommand' -a 'status' -d 'Show all accounts status'
complete -c cx -n '__fish_use_subcommand' -a 'dashboard' -d 'Live TUI dashboard'
complete -c cx -n '__fish_use_subcommand' -a 'web' -d 'Browser dashboard on localhost'
complete -c cx -n '__fish_use_subcommand' -a 'usage' -d 'Analyze token usage and costs'
complete -c cx -n '__fish_use_subcommand' -a 'doctor' -d 'Run health checks'
complete -c cx -n '__fish_use_subcommand' -a 'sync' -d 'Sync config to secondaries'
complete -c cx -n '__fish_use_subcommand' -a 'login' -d 'Re-authenticate an account'
complete -c cx -n '__fish_use_subcommand' -a 'daemon' -d 'Auto-switch + config-sync daemon'
complete -c cx -n '__fish_use_subcommand' -a 'completion' -d 'Output shell completion script'
complete -c cx -n '__fish_use_subcommand' -a 'insights' -d 'Usage pattern analysis'
complete -c cx -n '__fish_use_subcommand' -a 'predict' -d 'Forecast rate limit exhaustion'
complete -c cx -n '__fish_use_subcommand' -a 'statusline' -d 'CC status bar integration'
complete -c cx -n '__fish_use_subcommand' -a 'version' -d 'Print version information'
complete -c cx -n '__fish_use_subcommand' -a 'help' -d 'Show help message'

# switch completions (account names)
`)
	for _, name := range accounts {
		sb.WriteString(fmt.Sprintf("complete -c cx -n '__fish_seen_subcommand_from switch' -a '%s'\n", name))
		sb.WriteString(fmt.Sprintf("complete -c cx -n '__fish_seen_subcommand_from config' -a '%s'\n", name))
	}

	sb.WriteString(`
# usage completions (modes)
complete -c cx -n '__fish_seen_subcommand_from usage' -a 'daily weekly monthly session blocks messages'

# completion shell types
complete -c cx -n '__fish_seen_subcommand_from completion' -a 'bash fish powershell'

# run flags
complete -c cx -n '__fish_seen_subcommand_from run' -l 'prefer' -d 'Prefer a specific account'
complete -c cx -n '__fish_seen_subcommand_from run' -l 'balance' -d 'Round-robin selection'
`)
	return sb.String()
}

func powershellCompletion(reg *config.Registry) string {
	accounts := sortedAccountNames(reg)
	accountList := "'" + strings.Join(accounts, "', '") + "'"
	if len(accounts) == 0 {
		accountList = ""
	}

	return fmt.Sprintf(`# PowerShell completion for cx
# Add to your $PROFILE: cx completion powershell | Invoke-Expression

Register-ArgumentCompleter -CommandName cx -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    $commands = @('setup', 'switch', 'run', 'config', 'sessions', 'resume', 'status', 'daemon', 'dashboard', 'web', 'usage', 'insights', 'predict', 'doctor', 'sync', 'login', 'completion', 'statusline', 'version', 'help')
    $accounts = @(%s)
    $usageModes = @('daily', 'weekly', 'monthly', 'session', 'blocks', 'messages')
    $shells = @('bash', 'fish', 'powershell')

    $elements = $commandAst.CommandElements
    $count = $elements.Count

    if ($count -le 2) {
        # Complete subcommand
        $commands | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
        }
    } elseif ($count -ge 2) {
        $subcommand = $elements[1].ToString()
        switch ($subcommand) {
            'switch' {
                $accounts | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                    [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
                }
            }
            'usage' {
                $usageModes | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                    [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
                }
            }
            'completion' {
                $shells | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                    [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
                }
            }
        }
    }
}
`, accountList)
}
