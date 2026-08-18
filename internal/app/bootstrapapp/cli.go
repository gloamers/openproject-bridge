package bootstrapapp

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/gloamers/openproject-bridge/internal/defaults"
)

// Main parses flags and runs bootstrap. Returns a process exit code.
func Main(args []string) int {
	fs := flag.NewFlagSet("op-bootstrap", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var opts Options
	fs.StringVar(&opts.ConfigPath, "config", defaults.ConfigPath, "path to ecosystem YAML")
	fs.StringVar(&opts.OrgID, "org", "", "organization id (default: all)")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "plan only; do not create projects")
	fs.DurationVar(&opts.Timeout, "timeout", defaults.OPHTTPTimeout, "OpenProject HTTP timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	opts.Out = os.Stdout

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	app, err := New().WithOptions(opts).WithLogger(log).Build()
	if err != nil {
		log.Error("load config failed", slog.String("err", err.Error()))
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if err := app.Run(context.Background()); err != nil {
		log.Error("bootstrap failed", slog.String("err", err.Error()))
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
