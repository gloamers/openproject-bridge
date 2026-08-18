package bootstrap_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gloamers/openproject-bridge/internal/bootstrap"
	"github.com/gloamers/openproject-bridge/internal/config"
	"github.com/gloamers/openproject-bridge/internal/opclient"
)

type fakeAPI struct {
	byID    map[string]*opclient.Project
	creates int
	nextID  int
}

func (f *fakeAPI) GetByIdentifier(_ context.Context, identifier string) (*opclient.Project, error) {
	if p, ok := f.byID[identifier]; ok {
		cp := *p
		return &cp, nil
	}
	return nil, opclient.ErrNotFound
}

func (f *fakeAPI) CreateProject(_ context.Context, in opclient.CreateProjectInput) (*opclient.Project, error) {
	f.creates++
	f.nextID++
	p := &opclient.Project{ID: f.nextID, Identifier: in.Identifier, Name: in.Name, Active: true}
	f.byID[in.Identifier] = p
	return p, nil
}

func testOrg() *config.Organization {
	return &config.Organization{
		ID: "acme",
		Parent: config.ParentProject{
			Identifier:  "acme",
			Name:        "Acme",
			Description: "parent",
		},
		Products: []config.Product{
			{Identifier: "core", Name: "Core", GitHub: "acme/core"},
			{Identifier: "cli", Name: "CLI", GitHub: "acme/cli"},
		},
	}
}

func TestEnsureCreatesAll(t *testing.T) {
	t.Parallel()

	api := &fakeAPI{byID: map[string]*opclient.Project{}, nextID: 100}
	rows, err := bootstrap.EnsureOrganization(context.Background(), api, testOrg(), bootstrap.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if api.creates != 3 {
		t.Fatalf("creates = %d", api.creates)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d", len(rows))
	}
	for _, r := range rows {
		if r.Status != bootstrap.StatusCreated {
			t.Fatalf("row %#v", r)
		}
	}
}

func TestEnsureIdempotent(t *testing.T) {
	t.Parallel()

	api := &fakeAPI{
		byID: map[string]*opclient.Project{
			"acme": {ID: 1, Identifier: "acme", Name: "Acme"},
			"core": {ID: 2, Identifier: "core", Name: "Core"},
			"cli":  {ID: 3, Identifier: "cli", Name: "CLI"},
		},
		nextID: 3,
	}
	rows, err := bootstrap.EnsureOrganization(context.Background(), api, testOrg(), bootstrap.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if api.creates != 0 {
		t.Fatalf("creates = %d", api.creates)
	}
	for _, r := range rows {
		if r.Status != bootstrap.StatusExists {
			t.Fatalf("row %#v", r)
		}
	}
}

func TestEnsureDryRun(t *testing.T) {
	t.Parallel()

	api := &fakeAPI{byID: map[string]*opclient.Project{}}
	rows, err := bootstrap.EnsureOrganization(context.Background(), api, testOrg(), bootstrap.Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if api.creates != 0 {
		t.Fatalf("creates = %d", api.creates)
	}
	for _, r := range rows {
		if r.Status != bootstrap.StatusWould {
			t.Fatalf("row %#v", r)
		}
	}

	var b strings.Builder
	if err := bootstrap.WriteReport(&b, rows); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "would-create") {
		t.Fatalf("report = %s", b.String())
	}
}

func TestEnsurePropagatesAPIError(t *testing.T) {
	t.Parallel()

	api := &errAPI{err: errors.New("boom")}
	_, err := bootstrap.EnsureOrganization(context.Background(), api, testOrg(), bootstrap.Options{})
	if err == nil {
		t.Fatal("expected error")
	}
}

type errAPI struct{ err error }

func (e *errAPI) GetByIdentifier(context.Context, string) (*opclient.Project, error) {
	return nil, e.err
}
func (e *errAPI) CreateProject(context.Context, opclient.CreateProjectInput) (*opclient.Project, error) {
	return nil, e.err
}
