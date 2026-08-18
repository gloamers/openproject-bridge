package bridge

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gloamers/openproject-bridge/internal/defaults"
)

// Main parses flags, builds the app, and runs it. Returns a process exit code.
func Main(args []string) int {
	fs := flag.NewFlagSet("op-bridge", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var opts Options
	var logFormat string
	fs.StringVar(&opts.ConfigPath, "config", defaults.ConfigPath, "ecosystem YAML")
	fs.StringVar(&opts.Listen, "listen", "", "override bridge.webhook.listen")
	fs.StringVar(&opts.WebhookPath, "webhook-path", "", "override bridge.webhook.path")
	fs.StringVar(&opts.WebhookSecret, "webhook-secret", "", "override HMAC secret (else config/env)")
	fs.StringVar(&opts.MappingDB, "mapping-db", "", "override bridge.mapping_db")
	fs.StringVar(&opts.ADRDir, "adr-dir", defaults.ADRDir, "ADR output directory")
	fs.StringVar(&logFormat, "log-format", "text", "text|json")
	fs.DurationVar(&opts.ShutdownTimeout, "shutdown-timeout", defaults.ShutdownTimeout, "graceful shutdown timeout")
	fs.DurationVar(&opts.OPHTTPTimeout, "op-timeout", defaults.OPHTTPTimeout, "OpenProject HTTP timeout")
	fs.BoolVar(&opts.ReconcileOnce, "reconcile", false, "run closed-issue reconcile once and exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	log := newLogger(logFormat)
	app, err := New().WithOptions(opts).WithLogger(log).Build()
	if err != nil {
		log.Error("build failed", slog.String("err", err.Error()))
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	defer app.Close()

	if opts.ReconcileOnce {
		n, err := app.RunReconcile(context.Background())
		if err != nil {
			log.Error("reconcile failed", slog.String("err", err.Error()))
			return 1
		}
		log.Info("reconcile done", slog.Int("updated", n))
		return 0
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx); err != nil {
		log.Error("run failed", slog.String("err", err.Error()))
		return 1
	}
	return 0
}

func newLogger(format string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if format == "json" {
		return slog.New(slog.NewJSONHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, opts))
}
