package knowledge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gloamers/openproject-bridge/internal/config"
	"github.com/gloamers/openproject-bridge/internal/defaults"
	"github.com/gloamers/openproject-bridge/internal/domain"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Service writes ADR stubs and notifies GitHub (durable knowledge layer).
type Service struct {
	ADRDir string
	GitHub IssueNotifier
	Label  string
}

func (s *Service) adrDir() string {
	if s.ADRDir != "" {
		return s.ADRDir
	}
	return defaults.ADRDir
}

func (s *Service) documentedLabel() string {
	if s.Label != "" {
		return s.Label
	}
	return defaults.LabelDocumented
}

// OnDocumented implements sync.Documenter.
func (s *Service) OnDocumented(ctx context.Context, _ *config.GitHubRoute, issue domain.IssueEvent, wpURL string) error {
	owner, repo := issue.OwnerRepo()
	path, err := s.WriteADR(issue.Issue.Number, issue.Issue.Title, issue.Issue.HTMLURL, wpURL)
	if err != nil {
		return err
	}
	return s.notifyGitHub(ctx, owner, repo, issue.Issue.Number, path, wpURL)
}

// WriteADR creates docs/adr/NNNN-slug.md and returns relative path.
func (s *Service) WriteADR(number int, title, issueURL, wpURL string) (string, error) {
	dir := s.adrDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	slug := slugify(title)
	if slug == "" {
		slug = "decision"
	}
	name := fmt.Sprintf("%04d-%s.md", number, slug)
	path := filepath.Join(dir, name)
	content := fmt.Sprintf(`# %s

- Status: Accepted
- Date: %s
- GitHub: %s
- OpenProject: %s

## Context

(Issue discussion — see GitHub link.)

## Decision

(Fill in the durable decision.)

## Consequences

(Optional.)
`, title, time.Now().UTC().Format("2006-01-02"), issueURL, wpURL)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// PromoteIssue is a CLI helper: write ADR + optional GH comment for owner/repo#n.
func (s *Service) PromoteIssue(ctx context.Context, owner, repo string, number int, title, issueURL, wpURL string) (string, error) {
	path, err := s.WriteADR(number, title, issueURL, wpURL)
	if err != nil {
		return "", err
	}
	if err := s.notifyGitHub(ctx, owner, repo, number, path, wpURL); err != nil {
		return path, err
	}
	return path, nil
}

func (s *Service) notifyGitHub(ctx context.Context, owner, repo string, number int, path, wpURL string) error {
	if s.GitHub == nil {
		return nil
	}
	body := fmt.Sprintf("Documented decision recorded.\n\n- ADR: `%s`\n- OpenProject: %s", path, wpURL)
	if err := s.GitHub.CreateComment(ctx, owner, repo, number, body); err != nil {
		return err
	}
	_ = s.GitHub.AddLabel(ctx, owner, repo, number, s.documentedLabel())
	return nil
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonSlug.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 60 {
		s = s[:60]
	}
	return s
}
