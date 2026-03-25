package main

import (
	"fmt"
	"os"
	"runtime"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "setup":
		runSetup()
	case "run":
		runRun()
	case "statusline":
		runStatusline()
	case "init":
		runInit()
	case "login":
		runLogin()
	case "switch":
		runSwitch()
	case "sync":
		runSync()
	case "status":
		runStatus()
	case "usage":
		runUsage()
	case "completion":
		runCompletion()
	case "doctor":
		runDoctor()
	case "watch":
		runWatch()
	case "dashboard":
		runDashboard()
	case "web":
		runWeb()
	case "--version", "version":
		printVersion()
	case "--help", "help":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		fmt.Fprintln(os.Stderr, "Run 'cx help' for usage.")
		os.Exit(1)
	}
}

func printVersion() {
	fmt.Printf("cx %s (%s, %s/%s)\n", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

func printHelp() {
	fmt.Print(`cx - Claude Code multi-account manager

Usage:
  cx <command> [options]

Getting Started:
  setup              One-time interactive setup (accounts + shell wrapper)

Daily Use:
  switch <name>      Switch account (via wrapper: cx switch 5x)
  run [-- args]      Auto-select best account and launch claude
  status             All accounts: auth status + rate limits

Monitoring:
  dashboard          Live TUI dashboard
  web [--port N]     Browser dashboard on localhost
  usage <mode>       Usage analysis (daily/weekly/monthly/session/blocks/messages)

Maintenance:
  doctor             Health check all accounts
  sync [--force]     Sync config to secondary accounts
  login [name]       Re-authenticate an account
  init <name>        Create a new account directory
  watch              Auto-sync daemon (30s interval)
  completion <shell> Tab completion (bash/fish/powershell)
  statusline         CC status bar integration (internal)

  version            Print version
  help               Show this help
`)
}
