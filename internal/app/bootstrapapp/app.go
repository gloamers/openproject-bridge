// Package bootstrapapp wires and runs the op-bootstrap CLI.
package bootstrapapp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/gloamers/openproject-bridge/internal/bootstrap"
	"github.com/gloamers/openproject-bridge/internal/config"
	"github.com/gloamers/openproject-bridge/internal/defaults"
	"github.com/gloamers/openproject-bridge/internal/opclient"
)

// Options configure bootstrap.
type Options struct {
	ConfigPath string
	OrgID      string
	DryRun     bool
	Timeout    time.Duration
	Out        io.Writer
}

// Builder constructs a runnable bootstrap App.
type Builder struct {
	opts Options
	log  *slog.Logger
	root *config.Root
}

// New returns a builder with defaults.
func New() *Builder {
	return &Builder{
		opts: Options{
			ConfigPath: defaults.ConfigPath,
			Timeout:    defaults.OPHTTPTimeout,
			Out:        os.Stdout,
		},
	}
}

// WithOptions sets options.
func (b *Builder) WithOptions(o Options) *Builder {
	b.opts = o
	if b.opts.ConfigPath == "" {
		b.opts.ConfigPath = defaults.ConfigPath
	}
	if b.opts.Timeout <= 0 {
		b.opts.Timeout = defaults.OPHTTPTimeout
	}
	if b.opts.Out == nil {
		b.opts.Out = os.Stdout
	}
	return b
}

// WithLogger sets the logger.
func (b *Builder) WithLogger(log *slog.Logger) *Builder {
	b.log = log
	return b
}

// WithRoot injects config.
func (b *Builder) WithRoot(root *config.Root) *Builder {
	b.root = root
	return b
}

// Build loads config and returns App.
func (b *Builder) Build() (*App, error) {
	log := b.log
	if log == nil {
		log = slog.Default()
	}
	root := b.root
	if root == nil {
		var err error
		root, err = config.Load(b.opts.ConfigPath)
		if err != nil {
			return nil, err
		}
	}
	return &App{opts: b.opts, log: log, root: root}, nil
}

// App runs organization ensure.
type App struct {
	opts Options
	log  *slog.Logger
	root *config.Root
}

// Run ensures projects and writes the report.
func (a *App) Run(ctx context.Context) error {
	orgs := a.root.Organizations
	if a.opts.OrgID != "" {
		org, err := a.root.OrgByID(a.opts.OrgID)
		if err != nil {
			return err
		}
		orgs = []config.Organization{*org}
	}

	var all []bootstrap.Row
	for i := range orgs {
		org := &orgs[i]
		apiKey, err := org.ResolveAPIKey()
		if err != nil {
			if a.opts.DryRun {
				a.log.Warn("dry-run: API key missing; project lookups/creates skipped for org",
					slog.String("org", org.ID),
					slog.String("err", err.Error()),
				)
				all = append(all, dryRunRowsWithoutAPI(org)...)
				continue
			}
			return fmt.Errorf("org %s: %w", org.ID, err)
		}

		client := opclient.New(org.OpenProject.URL, apiKey, a.opts.Timeout)
		a.log.Info("bootstrapping organization",
			slog.String("org", org.ID),
			slog.String("url", client.BaseURL()),
			slog.Bool("dry_run", a.opts.DryRun),
		)

		rows, err := bootstrap.EnsureOrganization(ctx, client, org, bootstrap.Options{DryRun: a.opts.DryRun})
		if err != nil {
			return fmt.Errorf("org %s: %w", org.ID, err)
		}
		all = append(all, rows...)
	}
	return bootstrap.WriteReport(a.opts.Out, all)
}

func dryRunRowsWithoutAPI(org *config.Organization) []bootstrap.Row {
	rows := []bootstrap.Row{{
		OrgID:      org.ID,
		Identifier: org.Parent.Identifier,
		Kind:       "parent",
		Status:     bootstrap.StatusWould,
	}}
	for _, p := range org.Products {
		rows = append(rows, bootstrap.Row{
			OrgID:      org.ID,
			Identifier: p.Identifier,
			GitHub:     p.GitHub,
			Kind:       "product",
			Status:     bootstrap.StatusWould,
		})
	}
	return rows
}
