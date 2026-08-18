// Package documentapp wires and runs the op-document CLI.
package documentapp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/gloamers/openproject-bridge/internal/config"
	"github.com/gloamers/openproject-bridge/internal/defaults"
	"github.com/gloamers/openproject-bridge/internal/githubclient"
	"github.com/gloamers/openproject-bridge/internal/knowledge"
	"github.com/gloamers/openproject-bridge/internal/storage"
	"github.com/gloamers/openproject-bridge/internal/storage/sqlite"
)

// Options configure document promotion.
type Options struct {
	ConfigPath string
	Issue      string // owner/repo#number
	Title      string
	WPURL      string
	ADRDir     string
	NoGitHub   bool
	Out        io.Writer
}

// Builder constructs a document App.
type Builder struct {
	opts      Options
	log       *slog.Logger
	root      *config.Root
	store     IssueStore
	storeOpen OpenFunc
	github    *githubclient.Client
}

// New returns a builder with defaults.
func New() *Builder {
	return &Builder{
		opts: Options{
			ConfigPath: defaults.ConfigPath,
			ADRDir:     defaults.ADRDir,
			Out:        os.Stdout,
		},
		storeOpen: func(path string) (IssueStore, error) {
			return sqlite.Open(path)
		},
	}
}

// WithOptions sets options.
func (b *Builder) WithOptions(o Options) *Builder {
	b.opts = o
	if b.opts.ConfigPath == "" {
		b.opts.ConfigPath = defaults.ConfigPath
	}
	if b.opts.ADRDir == "" {
		b.opts.ADRDir = defaults.ADRDir
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

// WithStore injects storage (for WP URL lookup).
func (b *Builder) WithStore(store IssueStore) *Builder {
	b.store = store
	return b
}

// WithStoreOpen sets storage factory.
func (b *Builder) WithStoreOpen(open OpenFunc) *Builder {
	if open != nil {
		b.storeOpen = open
	}
	return b
}

// WithGitHub injects GitHub client.
func (b *Builder) WithGitHub(gh *githubclient.Client) *Builder {
	b.github = gh
	return b
}

// Build returns a ready App.
func (b *Builder) Build() (*App, error) {
	log := b.log
	if log == nil {
		log = slog.Default()
	}
	if strings.TrimSpace(b.opts.Issue) == "" {
		return nil, fmt.Errorf("-issue owner/repo#n is required")
	}
	owner, repo, number, err := parseIssue(b.opts.Issue)
	if err != nil {
		return nil, err
	}

	root := b.root
	if root == nil {
		root, err = config.Load(b.opts.ConfigPath)
		if err != nil {
			log.Warn("config not loaded; continuing ADR-only", slog.String("err", err.Error()))
			root = &config.Root{}
		}
	}

	gh := b.github
	if gh == nil && !b.opts.NoGitHub {
		if tok, err := config.ResolveSecret(defaults.EnvGitHubToken); err == nil {
			gh = &githubclient.Client{Token: tok}
		} else {
			log.Warn(defaults.EnvGitHubToken + " missing; ADR only")
		}
	}

	return &App{
		opts:   b.opts,
		log:    log,
		root:   root,
		store:  b.store,
		open:   b.storeOpen,
		github: gh,
		owner:  owner,
		repo:   repo,
		number: number,
	}, nil
}

// App promotes an issue to an ADR.
type App struct {
	opts   Options
	log    *slog.Logger
	root   *config.Root
	store  IssueStore
	open   OpenFunc
	github *githubclient.Client
	owner  string
	repo   string
	number int
}

// Run writes the ADR and optional GitHub notification.
func (a *App) Run(ctx context.Context) error {
	titleStr := a.opts.Title
	if titleStr == "" {
		titleStr = fmt.Sprintf("%s/%s#%d", a.owner, a.repo, a.number)
	}
	issueURL := fmt.Sprintf("https://github.com/%s/%s/issues/%d", a.owner, a.repo, a.number)
	wp := a.opts.WPURL
	if wp == "" {
		wp = a.lookupWPURL(ctx)
	}

	svc := &knowledge.Service{
		ADRDir: a.opts.ADRDir,
		GitHub: a.github,
		Label:  a.root.DocumentedLabel(),
	}
	path, err := svc.PromoteIssue(ctx, a.owner, a.repo, a.number, titleStr, issueURL, wp)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(a.opts.Out, path)
	return err
}

func (a *App) lookupWPURL(ctx context.Context) string {
	if a.store != nil {
		if m, err := a.store.GetIssue(ctx, storage.IssueKey{Owner: a.owner, Repo: a.repo, Number: a.number}); err == nil && m != nil {
			return m.WPURL
		}
		return ""
	}
	db := a.root.Bridge.MappingDB
	if db == "" || a.open == nil {
		return ""
	}
	store, err := a.open(db)
	if err != nil {
		return ""
	}
	defer store.Close()
	m, err := store.GetIssue(ctx, storage.IssueKey{Owner: a.owner, Repo: a.repo, Number: a.number})
	if err != nil || m == nil {
		return ""
	}
	return m.WPURL
}

func parseIssue(s string) (owner, repo string, number int, err error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, "#")
	if len(parts) != 2 {
		return "", "", 0, fmt.Errorf("issue must be owner/repo#number")
	}
	or := strings.Split(parts[0], "/")
	if len(or) != 2 {
		return "", "", 0, fmt.Errorf("issue must be owner/repo#number")
	}
	var n int
	if _, err := fmt.Sscanf(parts[1], "%d", &n); err != nil || n <= 0 {
		return "", "", 0, fmt.Errorf("invalid issue number")
	}
	return or[0], or[1], n, nil
}
