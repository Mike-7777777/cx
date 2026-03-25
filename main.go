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
	case "run":
		runRun()
	case "statusline":
		runStatusline()
	case "init":
		runInit()
	case "switch":
		runSwitch()
	case "sync":
		runSync()
	case "status":
		runStatus()
	case "usage":
		runUsage()
	case "--version", "version":
		printVersion()
	case "--help", "help":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		fmt.Fprintln(os.Stderr, "Run 'cc-monitor help' for usage.")
		os.Exit(1)
	}
}

func printVersion() {
	fmt.Printf("cc-monitor %s (%s, %s/%s)\n", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

func printHelp() {
	fmt.Print(`cc-monitor - Claude Code account manager

Usage:
  cc-monitor <command> [options]

Commands:
  run           Auto-select best account and launch claude
  statusline    Print a compact status line (for shell prompts / tmux)
  init          Initialize cc-monitor configuration
  switch        Switch active Claude Code account
  sync          Sync account state
  status        Show full account status
  usage         Analyze token usage and costs

  version       Print version information
  help          Show this help message
`)
}
