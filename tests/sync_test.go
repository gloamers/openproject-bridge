//go:build integration

package tests

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/gloamers/openproject-bridge/internal/bootstrap"
	"github.com/gloamers/openproject-bridge/internal/config"
	"github.com/gloamers/openproject-bridge/internal/domain"
	"github.com/gloamers/openproject-bridge/internal/opclient"
	"github.com/gloamers/openproject-bridge/internal/storage"
	"github.com/gloamers/openproject-bridge/internal/storage/sqlite"
	brsync "github.com/gloamers/openproject-bridge/internal/sync"
)

func TestIntegration_SyncIssueOpenedClosed(t *testing.T) {
	baseURL, apiKey := resolveItestOpenProject(t)
	ctx := context.Background()
	client := opclient.New(baseURL, apiKey, 60*time.Second)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000)
	parentID := "br-sync-p-" + suffix
	childID := "br-sync-c-" + suffix
	org := &config.Organization{
		ID: "itest",
		OpenProject: config.OpenProjectCreds{
			URL: baseURL,
		},
		Defaults: config.OrgDefaults{
			Sync:      config.SyncDefaults{Issues: true, AttachSpec: true},
			StatusMap: map[string]string{"opened": "New", "closed": "Closed"},
		},
		Parent: config.ParentProject{
			Identifier: parentID,
			Name:       "Sync Parent " + suffix,
		},
		Products: []config.Product{{
			Identifier: childID,
			Name:       "Sync Child " + suffix,
			GitHub:     "itest/sync-" + suffix,
		}},
	}
	if _, err := bootstrap.EnsureOrganization(ctx, client, org, bootstrap.Options{}); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	mapRepo, err := sqlite.Open(filepath.Join(t.TempDir(), "m.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mapRepo.Close() })

	root := &config.Root{
		Organizations: []config.Organization{*org},
		Bridge:        config.Bridge{Labels: config.Labels{Documented: "documented"}},
	}
	svc := &brsync.Service{
		Root:   root,
		Issues: mapRepo,
		Clients: func(o *config.Organization) (brsync.OpenProject, error) {
			return opclient.New(o.OpenProject.URL, apiKey, 60*time.Second), nil
		},
	}

	repo := "itest/sync-" + suffix
	opened := []byte(fmt.Sprintf(`{
	  "action":"opened",
	  "issue":{"number":1,"title":"Itest bug","body":"# hello\n","html_url":"https://github.com/%s/issues/1","labels":[]},
	  "repository":{"full_name":%q,"name":"sync-%s","owner":{"login":"itest"}}
	}`, repo, repo, suffix))
	if err := svc.Handle(ctx, domain.Delivery{Event: "issues", DeliveryID: "itest-open-" + suffix, Body: opened}); err != nil {
		t.Fatalf("opened: %v", err)
	}
	m, err := mapRepo.GetIssue(ctx, storage.IssueKey{Owner: "itest", Repo: "sync-" + suffix, Number: 1})
	if err != nil || m == nil || m.WPID <= 0 {
		t.Fatalf("mapping %#v err=%v", m, err)
	}
	wp, err := client.GetWorkPackage(ctx, m.WPID)
	if err != nil {
		t.Fatalf("get wp: %v", err)
	}
	if wp.Subject == "" {
		t.Fatal("empty subject")
	}

	closed := []byte(fmt.Sprintf(`{
	  "action":"closed",
	  "issue":{"number":1,"title":"Itest bug","html_url":"https://github.com/%s/issues/1"},
	  "repository":{"full_name":%q,"name":"sync-%s","owner":{"login":"itest"}}
	}`, repo, repo, suffix))
	if err := svc.Handle(ctx, domain.Delivery{Event: "issues", DeliveryID: "itest-close-" + suffix, Body: closed}); err != nil {
		t.Fatalf("closed: %v", err)
	}
}
