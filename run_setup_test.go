package main

import (
	"testing"

	"github.com/Mike-7777777/cx/internal/platform"
)

func TestFindWrapperEnd_Bash(t *testing.T) {
	content := `# some profile stuff
export PATH=$PATH:/usr/local/bin

# cx: Claude Code multi-account wrapper
cx() {
    case "$1" in
        switch) eval "$(command cx switch "$2")" ;;
        *) command cx "$@" ;;
    esac
}

# other stuff below
alias ll='ls -la'
`
	idx := 43 // position of "# cx:"
	end := findWrapperEnd(content, idx, platform.ShellBash)
	remaining := content[end:]
	if remaining != "# other stuff below\nalias ll='ls -la'\n" {
		t.Errorf("unexpected remaining after bash wrapper end: %q", remaining)
	}
}

func TestFindWrapperEnd_PowerShell(t *testing.T) {
	content := `# cx: Claude Code multi-account wrapper (path set by cx setup)
$cxExe = "/path/to/cx.exe"
function cx {
    if ($args[0] -eq "switch") {
        $cmd = & $cxExe switch $args[1] --shell=powershell
        Invoke-Expression ($cmd -join "` + "`" + `n")
    } else {
        & $cxExe @args
    }
}
`
	idx := 0
	end := findWrapperEnd(content, idx, platform.ShellPowerShell)
	remaining := content[end:]
	if remaining != "" {
		t.Errorf("expected empty remaining for PowerShell, got %q", remaining)
	}
}

func TestFindWrapperEnd_Fish(t *testing.T) {
	content := `# cx: Claude Code multi-account wrapper
function cx
    if test "$argv[1]" = "switch"
        eval (command cx switch $argv[2])
    else
        command cx $argv
    end
end

# other fish config
`
	idx := 0
	end := findWrapperEnd(content, idx, platform.ShellFish)
	remaining := content[end:]
	if remaining != "# other fish config\n" {
		t.Errorf("unexpected remaining after fish wrapper end: %q", remaining)
	}
}

func TestFindWrapperEnd_WrapperAtFileStart(t *testing.T) {
	// Edge case: wrapper is the only content, closing brace at column 0.
	content := "# cx: Claude Code multi-account wrapper\ncx() {\n    command cx \"$@\"\n}\n"
	idx := 0
	end := findWrapperEnd(content, idx, platform.ShellBash)
	if end != len(content) {
		t.Errorf("end=%d, want %d (full content length)", end, len(content))
	}
}
