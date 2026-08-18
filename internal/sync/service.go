package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gloamers/openproject-bridge/internal/config"
	"github.com/gloamers/openproject-bridge/internal/defaults"
	"github.com/gloamers/openproject-bridge/internal/domain"
	"github.com/gloamers/openproject-bridge/internal/opclient"
	"github.com/gloamers/openproject-bridge/internal/storage"
)

// Service handles GitHub issue events → OpenProject work packages.
type Service struct {
	Root     *config.Root
	Issues   IssueRepository
	Clients  ClientFactory
	Document Documenter
	Log      *slog.Logger
}

func (s *Service) log() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}

// Handle processes one delivery (idempotent via delivery_id claim done by webhook).
func (s *Service) Handle(ctx context.Context, d domain.Delivery) error {
	if d.Event != defaults.EventIssues {
		s.log().Debug("ignoring event", slog.String("event", d.Event))
		return nil
	}
	var ev domain.IssueEvent
	if err := json.Unmarshal(d.Body, &ev); err != nil {
		return fmt.Errorf("sync: decode issues payload: %w", err)
	}
	fullName := ev.RepoFullName()
	route, err := s.Root.ResolveGitHub(fullName)
	if err != nil {
		s.log().Info("no mapping for repo; skipping", slog.String("repo", fullName), slog.String("err", err.Error()))
		return nil
	}
	syncCfg := route.Org.EffectiveSync(route.Product)
	if !syncCfg.Issues {
		return nil
	}

	switch ev.Action {
	case defaults.ActionOpened:
		return s.onOpened(ctx, route, syncCfg, ev)
	case defaults.ActionClosed, defaults.ActionReopened:
		return s.onStatus(ctx, route, ev)
	case defaults.ActionLabeled:
		return s.onLabeled(ctx, route, ev)
	case "edited", "unlabeled", "assigned", "unassigned":
		// Corp default: do not mirror discussion edits.
		return nil
	default:
		return nil
	}
}

func (s *Service) onOpened(ctx context.Context, route *config.GitHubRoute, syncCfg config.SyncDefaults, ev domain.IssueEvent) error {
	owner, repo := ev.OwnerRepo()
	key := storage.IssueKey{Owner: owner, Repo: repo, Number: ev.Issue.Number}
	if existing, err := s.Issues.GetIssue(ctx, key); err != nil {
		return err
	} else if existing != nil {
		return nil
	}

	api, err := s.Clients(route.Org)
	if err != nil {
		return err
	}
	proj, err := api.GetByIdentifier(ctx, route.Product.Identifier)
	if err != nil {
		return fmt.Errorf("sync: resolve product %q: %w", route.Product.Identifier, err)
	}

	typeID := 0
	if name := typeFromLabels(route.Org.Defaults.TypeMap, ev); name != "" {
		if t, err := api.FindTypeByName(ctx, name); err == nil {
			typeID = t.ID
		}
	}
	statusID := 0
	if name := route.Org.MappedStatus(defaults.StatusKeyOpened); name != "" {
		if st, err := api.FindStatusByName(ctx, name); err == nil {
			statusID = st.ID
		}
	}

	subject := fmt.Sprintf("[%s#%d] %s", repo, ev.Issue.Number, ev.Issue.Title)
	desc := fmt.Sprintf("GitHub: %s\n\n<!-- managed by openproject-bridge -->", ev.Issue.HTMLURL)
	wp, err := api.CreateWorkPackage(ctx, opclient.CreateWorkPackageInput{
		ProjectID:   proj.ID,
		Subject:     subject,
		Description: desc,
		TypeID:      typeID,
		StatusID:    statusID,
	})
	if err != nil {
		return fmt.Errorf("sync: create WP: %w", err)
	}

	if syncCfg.AttachSpec && strings.TrimSpace(ev.Issue.Body) != "" {
		if err := api.AddAttachment(ctx, wp.ID, defaults.SpecAttachment, []byte(ev.Issue.Body), defaults.SpecAttachmentDesc); err != nil {
			s.log().Warn("attach spec failed", slog.String("file", defaults.SpecAttachment), slog.String("err", err.Error()), slog.Int("wp_id", wp.ID))
		}
	}

	return s.Issues.UpsertIssue(ctx, storage.IssueMapping{
		IssueKey:  key,
		OrgID:     route.Org.ID,
		ProjectID: proj.ID,
		WPID:      wp.ID,
		WPURL:     api.WorkPackageURL(wp.ID),
	})
}

func (s *Service) onStatus(ctx context.Context, route *config.GitHubRoute, ev domain.IssueEvent) error {
	owner, repo := ev.OwnerRepo()
	m, err := s.Issues.GetIssue(ctx, storage.IssueKey{Owner: owner, Repo: repo, Number: ev.Issue.Number})
	if err != nil || m == nil {
		return err
	}
	api, err := s.Clients(route.Org)
	if err != nil {
		return err
	}
	key := defaults.StatusKeyClosed
	if ev.Action == defaults.ActionReopened {
		key = defaults.StatusKeyOpened
	}
	name := route.Org.MappedStatus(key)
	st, err := api.FindStatusByName(ctx, name)
	if err != nil {
		return err
	}
	_, err = api.SetWorkPackageStatus(ctx, m.WPID, st.ID)
	return err
}

func (s *Service) onLabeled(ctx context.Context, route *config.GitHubRoute, ev domain.IssueEvent) error {
	docLabel := s.Root.DocumentedLabel()
	labelName := ""
	if ev.Label != nil {
		labelName = ev.Label.Name
	}
	if !strings.EqualFold(labelName, docLabel) {
		return nil
	}
	if s.Document == nil {
		return nil
	}
	owner, repo := ev.OwnerRepo()
	m, err := s.Issues.GetIssue(ctx, storage.IssueKey{Owner: owner, Repo: repo, Number: ev.Issue.Number})
	if err != nil || m == nil {
		return err
	}
	return s.Document.OnDocumented(ctx, route, ev, m.WPURL)
}

func typeFromLabels(typeMap map[string]string, ev domain.IssueEvent) string {
	for _, l := range ev.Issue.Labels {
		if v, ok := typeMap[strings.ToLower(l.Name)]; ok {
			return v
		}
		if v, ok := typeMap[l.Name]; ok {
			return v
		}
	}
	return ""
}
