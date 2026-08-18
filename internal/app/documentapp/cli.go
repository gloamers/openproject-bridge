package documentapp

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/gloamers/openproject-bridge/internal/defaults"
)

// Main parses flags and runs document promotion. Returns a process exit code.
func Main(args []string) int {
	fs := flag.NewFlagSet("op-document", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var opts Options
	fs.StringVar(&opts.ConfigPath, "config", defaults.ConfigPath, "ecosystem YAML")
	fs.StringVar(&opts.Issue, "issue", "", "owner/repo#number")
	fs.StringVar(&opts.Title, "title", "", "ADR title (default: issue ref)")
	fs.StringVar(&opts.WPURL, "wp-url", "", "OpenProject work package URL")
	fs.StringVar(&opts.ADRDir, "adr-dir", defaults.ADRDir, "ADR directory")
	fs.BoolVar(&opts.NoGitHub, "no-github", false, "only write ADR; skip GitHub comment/label")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	opts.Out = os.Stdout

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	app, err := New().WithOptions(opts).WithLogger(log).Build()
	if err != nil {
		log.Error("build failed", slog.String("err", err.Error()))
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if err := app.Run(context.Background()); err != nil {
		log.Error("promote failed", slog.String("err", err.Error()))
		return 1
	}
	return 0
}
