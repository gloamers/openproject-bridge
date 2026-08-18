package sync_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/gloamers/openproject-bridge/internal/config"
	"github.com/gloamers/openproject-bridge/internal/domain"
	"github.com/gloamers/openproject-bridge/internal/opclient"
	"github.com/gloamers/openproject-bridge/internal/storage/sqlite"
	brsync "github.com/gloamers/openproject-bridge/internal/sync"
)

type fakeOP struct {
	projects map[string]*opclient.Project
	statuses map[string]*opclient.NamedResource
	types    map[string]*opclient.NamedResource
	created  []*opclient.WorkPackage
	nextID   int
	statusOf map[int]int
	files    int
}

func (f *fakeOP) GetByIdentifier(_ context.Context, id string) (*opclient.Project, error) {
	p, ok := f.projects[id]
	if !ok {
		return nil, opclient.ErrNotFound
	}
	return p, nil
}
func (f *fakeOP) CreateWorkPackage(_ context.Context, in opclient.CreateWorkPackageInput) (*opclient.WorkPackage, error) {
	f.nextID++
	wp := &opclient.WorkPackage{ID: f.nextID, Subject: in.Subject, LockVersion: 1}
	f.created = append(f.created, wp)
	return wp, nil
}
func (f *fakeOP) SetWorkPackageStatus(_ context.Context, wpID, statusID int) (*opclient.WorkPackage, error) {
	f.statusOf[wpID] = statusID
	return &opclient.WorkPackage{ID: wpID, LockVersion: 2}, nil
}
func (f *fakeOP) FindStatusByName(_ context.Context, name string) (*opclient.NamedResource, error) {
	s, ok := f.statuses[name]
	if !ok {
		return nil, opclient.ErrNotFound
	}
	return s, nil
}
func (f *fakeOP) FindTypeByName(_ context.Context, name string) (*opclient.NamedResource, error) {
	s, ok := f.types[name]
	if !ok {
		return nil, opclient.ErrNotFound
	}
	return s, nil
}
func (f *fakeOP) AddAttachment(context.Context, int, string, []byte, string) error {
	f.files++
	return nil
}
func (f *fakeOP) WorkPackageURL(id int) string { return fmt.Sprintf("http://op/wp/%d", id) }

func TestServiceOpenedClosed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repo, err := sqlite.Open(filepath.Join(dir, "m.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	fake := &fakeOP{
		projects: map[string]*opclient.Project{"core": {ID: 9, Identifier: "core", Name: "Core"}},
		statuses: map[string]*opclient.NamedResource{
			"New":    {ID: 1, Name: "New"},
			"Closed": {ID: 2, Name: "Closed"},
		},
		types:    map[string]*opclient.NamedResource{"Bug": {ID: 3, Name: "Bug"}},
		statusOf: map[int]int{},
		nextID:   100,
	}

	root := &config.Root{
		Organizations: []config.Organization{{
			ID: "acme",
			Defaults: config.OrgDefaults{
				Sync:      config.SyncDefaults{Issues: true, AttachSpec: true},
				StatusMap: map[string]string{"opened": "New", "closed": "Closed"},
				TypeMap:   map[string]string{"bug": "Bug"},
			},
			Products: []config.Product{{
				Identifier: "core", Name: "Core", GitHub: "acme/core",
			}},
		}},
		Bridge: config.Bridge{Labels: config.Labels{Documented: "documented"}},
	}

	svc := &brsync.Service{
		Root:   root,
		Issues: repo,
		Clients: func(org *config.Organization) (brsync.OpenProject, error) {
			return fake, nil
		},
	}

	opened := []byte(`{
	  "action":"opened",
	  "issue":{"number":7,"title":"Crash","body":"# Spec\n","html_url":"https://github.com/acme/core/issues/7","labels":[{"name":"bug"}]},
	  "repository":{"full_name":"acme/core","name":"core","owner":{"login":"acme"}}
	}`)
	if err := svc.Handle(context.Background(), domain.Delivery{Event: "issues", DeliveryID: "d1", Body: opened}); err != nil {
		t.Fatal(err)
	}
	if len(fake.created) != 1 || fake.files != 1 {
		t.Fatalf("created=%d files=%d", len(fake.created), fake.files)
	}

	closed := []byte(`{
	  "action":"closed",
	  "issue":{"number":7,"title":"Crash","html_url":"https://github.com/acme/core/issues/7"},
	  "repository":{"full_name":"acme/core","name":"core","owner":{"login":"acme"}}
	}`)
	if err := svc.Handle(context.Background(), domain.Delivery{Event: "issues", DeliveryID: "d2", Body: closed}); err != nil {
		t.Fatal(err)
	}
	if fake.statusOf[101] != 2 {
		t.Fatalf("status map %#v", fake.statusOf)
	}
}
