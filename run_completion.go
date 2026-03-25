package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Mike-7777777/cx/internal/config"
)

func runCompletion() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: cx completion <bash|fish|powershell>")
		os.Exit(1)
	}

	shell := strings.ToLower(os.Args[2])
	switch shell {
	case "bash":
		fmt.Print(bashCompletion())
	case "fish":
		fmt.Print(fishCompletion())
	case "powershell":
		fmt.Print(powershellCompletion())
	default:
		fmt.Fprintf(os.Stderr, "unsupported shell: %s (supported: bash, fish, powershell)\n", shell)
		os.Exit(1)
	}
}

// accountNames loads registered account names from the registry.
func accountNames() []string {
	regPath, err := config.RegistryPath()
	if err != nil {
		return nil
	}
	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(reg.Accounts))
	for name := range reg.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func bashCompletion() string {
	accounts := accountNames()
	accountList := strings.Join(accounts, " ")

	return fmt.Sprintf(`# bash completion for cx
# Add to ~/.bashrc: eval "$(cx completion bash)"

_cx_completions() {
    local cur prev commands
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    commands="run statusline init switch sync status usage completion doctor version help"

    case "${prev}" in
        cx)
            COMPREPLY=( $(compgen -W "${commands}" -- "${cur}") )
            return 0
            ;;
        switch|init)
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

func fishCompletion() string {
	accounts := accountNames()

	var sb strings.Builder
	sb.WriteString(`# fish completion for cx
# Add to ~/.config/fish/completions/cx.fish

# Disable file completions by default
complete -c cx -f

# Subcommands
complete -c cx -n '__fish_use_subcommand' -a 'run' -d 'Auto-select best account and launch claude'
complete -c cx -n '__fish_use_subcommand' -a 'statusline' -d 'Print compact status line'
complete -c cx -n '__fish_use_subcommand' -a 'init' -d 'Initialize cx configuration'
complete -c cx -n '__fish_use_subcommand' -a 'switch' -d 'Switch active Claude Code account'
complete -c cx -n '__fish_use_subcommand' -a 'sync' -d 'Sync account state'
complete -c cx -n '__fish_use_subcommand' -a 'status' -d 'Show full account status'
complete -c cx -n '__fish_use_subcommand' -a 'usage' -d 'Analyze token usage and costs'
complete -c cx -n '__fish_use_subcommand' -a 'completion' -d 'Output shell completion script'
complete -c cx -n '__fish_use_subcommand' -a 'doctor' -d 'Run health checks'
complete -c cx -n '__fish_use_subcommand' -a 'version' -d 'Print version information'
complete -c cx -n '__fish_use_subcommand' -a 'help' -d 'Show help message'

# switch completions (account names)
`)
	for _, name := range accounts {
		sb.WriteString(fmt.Sprintf("complete -c cx -n '__fish_seen_subcommand_from switch' -a '%s'\n", name))
		sb.WriteString(fmt.Sprintf("complete -c cx -n '__fish_seen_subcommand_from init' -a '%s'\n", name))
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

func powershellCompletion() string {
	accounts := accountNames()
	accountList := "'" + strings.Join(accounts, "', '") + "'"
	if len(accounts) == 0 {
		accountList = ""
	}

	return fmt.Sprintf(`# PowerShell completion for cx
# Add to your $PROFILE: cx completion powershell | Invoke-Expression

Register-ArgumentCompleter -CommandName cx -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    $commands = @('run', 'statusline', 'init', 'switch', 'sync', 'status', 'usage', 'completion', 'doctor', 'version', 'help')
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
            'init' {
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
