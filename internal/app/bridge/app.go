// Package bridge wires and runs the op-bridge process (webhook + sync + optional reconcile).
package bridge

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/gloamers/openproject-bridge/internal/config"
	"github.com/gloamers/openproject-bridge/internal/defaults"
	"github.com/gloamers/openproject-bridge/internal/githubclient"
	"github.com/gloamers/openproject-bridge/internal/httpserver"
	"github.com/gloamers/openproject-bridge/internal/knowledge"
	"github.com/gloamers/openproject-bridge/internal/opclient"
	"github.com/gloamers/openproject-bridge/internal/reconcile"
	"github.com/gloamers/openproject-bridge/internal/storage/sqlite"
	brsync "github.com/gloamers/openproject-bridge/internal/sync"
	"github.com/gloamers/openproject-bridge/internal/webhook"
)

// Options are immutable inputs for the builder.
type Options struct {
	ConfigPath      string
	Listen          string
	WebhookPath     string
	WebhookSecret   string
	MappingDB       string
	ADRDir          string
	ShutdownTimeout time.Duration
	OPHTTPTimeout   time.Duration
	ReconcileOnce   bool
	Workers         int
}

// Builder constructs an App with optional overrides (tests / alternate storage).
type Builder struct {
	opts      Options
	log       *slog.Logger
	root      *config.Root
	store     Store
	storeOpen OpenFunc
	github    *githubclient.Client
	ownStore  bool
}

// New returns a builder with defaults.
func New() *Builder {
	return &Builder{
		opts: defaultOptions(),
		storeOpen: func(path string) (Store, error) {
			return sqlite.Open(path)
		},
	}
}

func defaultOptions() Options {
	return Options{
		ConfigPath:      defaults.ConfigPath,
		ADRDir:          defaults.ADRDir,
		ShutdownTimeout: defaults.ShutdownTimeout,
		OPHTTPTimeout:   defaults.OPHTTPTimeout,
		Workers:         defaults.WebhookWorkers,
	}
}

// WithOptions replaces all options.
func (b *Builder) WithOptions(o Options) *Builder {
	b.opts = o
	b.opts = normalizeOptions(b.opts)
	return b
}

func normalizeOptions(o Options) Options {
	if o.ConfigPath == "" {
		o.ConfigPath = defaults.ConfigPath
	}
	if o.Workers <= 0 {
		o.Workers = defaults.WebhookWorkers
	}
	if o.ShutdownTimeout <= 0 {
		o.ShutdownTimeout = defaults.ShutdownTimeout
	}
	if o.OPHTTPTimeout <= 0 {
		o.OPHTTPTimeout = defaults.OPHTTPTimeout
	}
	if o.ADRDir == "" {
		o.ADRDir = defaults.ADRDir
	}
	return o
}

// WithLogger sets the logger.
func (b *Builder) WithLogger(log *slog.Logger) *Builder {
	b.log = log
	return b
}

// WithRoot injects a pre-loaded config (skips file load).
func (b *Builder) WithRoot(root *config.Root) *Builder {
	if root != nil {
		root.ApplyDefaults()
	}
	b.root = root
	return b
}

// WithStore injects an already-open Store (skips open; caller owns Close).
func (b *Builder) WithStore(store Store) *Builder {
	b.store = store
	b.ownStore = false
	return b
}

// WithStoreOpen sets the storage driver factory (default: sqlite.Open).
func (b *Builder) WithStoreOpen(open OpenFunc) *Builder {
	if open != nil {
		b.storeOpen = open
	}
	return b
}

// WithGitHub injects a GitHub client.
func (b *Builder) WithGitHub(gh *githubclient.Client) *Builder {
	b.github = gh
	return b
}

// Build validates options, opens dependencies, and returns a ready App.
func (b *Builder) Build() (*App, error) {
	log := b.log
	if log == nil {
		log = slog.Default()
	}
	b.opts = normalizeOptions(b.opts)

	root := b.root
	if root == nil {
		var err error
		root, err = config.Load(b.opts.ConfigPath)
		if err != nil {
			return nil, fmt.Errorf("load config: %w", err)
		}
	} else {
		root.ApplyDefaults()
	}

	store := b.store
	ownStore := b.ownStore
	dbPath := resolveDBPath(root.Bridge.MappingDB, b.opts.MappingDB)
	if store == nil {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
			return nil, fmt.Errorf("mkdir mapping db: %w", err)
		}
		var err error
		store, err = b.storeOpen(dbPath)
		if err != nil {
			return nil, fmt.Errorf("open storage: %w", err)
		}
		ownStore = true
	}

	gh := b.github
	if gh == nil {
		if t, err := config.ResolveSecret(defaults.EnvGitHubToken); err == nil {
			gh = &githubclient.Client{Token: t}
		}
	}

	timeout := b.opts.OPHTTPTimeout
	newOP := func(org *config.Organization) (brsync.OpenProject, error) {
		key, err := org.ResolveAPIKey()
		if err != nil {
			return nil, err
		}
		return opclient.New(org.OpenProject.URL, key, timeout), nil
	}

	docs := &knowledge.Service{
		ADRDir: b.opts.ADRDir,
		GitHub: gh,
		Label:  root.DocumentedLabel(),
	}
	syncSvc := &brsync.Service{
		Root:     root,
		Issues:   store,
		Clients:  newOP,
		Document: docs,
		Log:      log,
	}

	return &App{
		opts:     b.opts,
		log:      log,
		root:     root,
		store:    store,
		ownStore: ownStore,
		dbPath:   dbPath,
		github:   gh,
		newOP:    newOP,
		sync:     syncSvc,
	}, nil
}

// App is a built bridge process.
type App struct {
	opts     Options
	log      *slog.Logger
	root     *config.Root
	store    Store
	ownStore bool
	dbPath   string
	github   *githubclient.Client
	newOP    brsync.ClientFactory
	sync     *brsync.Service
}

// Close releases owned resources.
func (a *App) Close() error {
	if a.ownStore && a.store != nil {
		return a.store.Close()
	}
	return nil
}

// RunReconcile syncs closed GitHub issues to OpenProject once.
func (a *App) RunReconcile(ctx context.Context) (int, error) {
	if a.github == nil {
		return 0, fmt.Errorf("%s required for reconcile", defaults.EnvGitHubToken)
	}
	r := &reconcile.Service{
		Root:   a.root,
		Issues: a.store,
		GitHub: a.github,
		Clients: func(org *config.Organization) (reconcile.OpenProject, error) {
			return a.newOP(org)
		},
		Log: a.log,
	}
	return r.Run(ctx)
}

// Run starts the webhook HTTP server until ctx is canceled.
func (a *App) Run(ctx context.Context) error {
	secretBytes := []byte(a.opts.WebhookSecret)
	if len(secretBytes) == 0 {
		if s, err := a.root.FallbackWebhookSecret(); err == nil {
			secretBytes = []byte(s)
		}
	}

	wh := &webhook.Handler{
		Root:       a.root,
		Deliveries: a.store,
		Sync:       a.sync,
		Secret:     secretBytes,
		MaxBody:    a.root.Bridge.Webhook.MaxBodyBytes,
		Log:        a.log,
	}
	wh.StartWorkers(a.opts.Workers)
	defer wh.ShutdownWorkers()

	addr := a.opts.Listen
	if addr == "" {
		addr = a.root.Bridge.Webhook.Listen
	}
	path := a.opts.WebhookPath
	if path == "" {
		path = a.root.Bridge.Webhook.Path
	}

	mux := httpserver.NewMux(httpserver.Options{
		WebhookPath: path,
		Webhook:     wh,
	})
	srv := httpserver.New(httpserver.Config{
		Addr:            addr,
		ShutdownTimeout: a.opts.ShutdownTimeout,
	}, mux)

	if err := srv.Start(); err != nil {
		return err
	}
	a.log.Info("op-bridge listening",
		slog.String("addr", srv.Addr()),
		slog.String("webhook_path", path),
		slog.String("mapping_db", a.dbPath),
	)

	<-ctx.Done()
	a.log.Info("shutdown signal received")
	shCtx, cancel := context.WithTimeout(context.Background(), a.opts.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shCtx); err != nil {
		return err
	}
	a.log.Info("server shut down cleanly")
	return nil
}

func resolveDBPath(fromConfig, override string) string {
	switch {
	case override != "":
		return override
	case fromConfig != "":
		return fromConfig
	default:
		return defaults.MappingDB
	}
}
