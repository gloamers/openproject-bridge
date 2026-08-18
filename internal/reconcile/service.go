package reconcile

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gloamers/openproject-bridge/internal/config"
	"github.com/gloamers/openproject-bridge/internal/defaults"
)

// Service best-effort reconciles closed GitHub issues → OP Closed status.
type Service struct {
	Root    *config.Root
	Issues  IssueRepository
	GitHub  GitHubIssues
	Clients ClientFactory
	Log     *slog.Logger
}

// Run scans mappings and updates OP status when GH issue is closed.
func (s *Service) Run(ctx context.Context) (updated int, err error) {
	log := s.Log
	if log == nil {
		log = slog.Default()
	}
	items, err := s.Issues.ListIssues(ctx)
	if err != nil {
		return 0, err
	}
	for _, m := range items {
		state, err := s.GitHub.GetIssue(ctx, m.Owner, m.Repo, m.Number)
		if err != nil {
			log.Warn("github get issue failed", slog.String("err", err.Error()))
			continue
		}
		if state != defaults.GitHubStateClosed {
			continue
		}
		org, err := s.Root.OrgByID(m.OrgID)
		if err != nil {
			continue
		}
		api, err := s.Clients(org)
		if err != nil {
			continue
		}
		name := org.MappedStatus(defaults.StatusKeyClosed)
		st, err := api.FindStatusByName(ctx, name)
		if err != nil {
			log.Warn("status lookup failed", slog.String("err", err.Error()))
			continue
		}
		if _, err := api.SetWorkPackageStatus(ctx, m.WPID, st.ID); err != nil {
			log.Warn("set status failed", slog.String("err", err.Error()))
			continue
		}
		updated++
		log.Info("reconciled closed issue",
			slog.String("issue", fmt.Sprintf("%s/%s#%d", m.Owner, m.Repo, m.Number)),
			slog.Int("wp_id", m.WPID),
		)
	}
	return updated, nil
}
