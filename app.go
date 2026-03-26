package main

import (
	"context"
	"io"
	"strings"

	"github.com/Mike-7777777/cx/internal/config"
)

// Runner is implemented by commands migrated to the testable pattern.
// Commands still using the legacy func() signature are dispatched directly
// by main.go — this interface is adopted incrementally.
type Runner interface {
	Run(ctx context.Context, app *App, args []string) error
}

// App holds shared dependencies injected into every command.
// Tests create App with buffer writers and mock registries;
// main() creates App with os.Stdout/Stderr and the real registry.
type App struct {
	Registry *config.Registry
	Stdout   io.Writer
	Stderr   io.Writer
	UseColor bool
}

// parseFlags extracts known flags from args and returns remaining positional args.
// Flags can be --key or --key=value. Returns a map of flag->value (bool flags get "true").
func parseFlags(args []string, known ...string) (flags map[string]string, positional []string) {
	flags = make(map[string]string)
	knownSet := make(map[string]bool, len(known))
	for _, k := range known {
		knownSet[k] = true
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "--") {
			positional = append(positional, a)
			continue
		}
		key := strings.TrimPrefix(a, "--")
		if eq := strings.IndexByte(key, '='); eq >= 0 {
			name := key[:eq]
			if knownSet[name] {
				flags[name] = key[eq+1:]
				continue
			}
		}
		if knownSet[key] {
			// Check if next arg is the value (not another flag).
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				flags[key] = args[i+1]
				i++
			} else {
				flags[key] = "true"
			}
			continue
		}
		positional = append(positional, a)
	}
	return flags, positional
}
