package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gloamers/openproject-bridge/internal/config"
)

func TestLoadExample(t *testing.T) {
	t.Parallel()

	root, err := config.Load(filepath.Join("..", "..", "configs", "ecosystem.example.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(root.Organizations) != 3 {
		t.Fatalf("orgs = %d", len(root.Organizations))
	}
	org, err := root.OrgByID("acme")
	if err != nil {
		t.Fatal(err)
	}
	if org.Parent.Identifier != "acme" {
		t.Fatalf("parent = %q", org.Parent.Identifier)
	}
	if org.GitHub.Org != "acme" {
		t.Fatalf("github.org = %q", org.GitHub.Org)
	}
	if len(org.Products) != 4 {
		t.Fatalf("products = %d", len(org.Products))
	}
	var app *config.Product
	for i := range org.Products {
		p := &org.Products[i]
		if p.ID == "core" && p.GitHub != "acme/core" {
			t.Fatalf("product core = %+v", p)
		}
		if p.ID == "acme" {
			app = p
		}
	}
	if app == nil || app.Identifier != "acme-app" || app.GitHub != "acme/acme" {
		t.Fatalf("product acme = %+v", app)
	}
	if _, err := root.OrgByID("globex"); err != nil {
		t.Fatal(err)
	}
	if _, err := root.OrgByID("client-x"); err != nil {
		t.Fatal(err)
	}
	if root.Organizations[0].OpenProject.URL != root.Organizations[1].OpenProject.URL {
		t.Fatal("acme and globex should share one OpenProject URL (Jira-style)")
	}
	if root.Organizations[0].OpenProject.URL == root.Organizations[2].OpenProject.URL {
		t.Fatal("client-x should use a separate OpenProject URL")
	}
}

func TestValidateDuplicateGitHub(t *testing.T) {
	t.Parallel()

	r := &config.Root{
		Organizations: []config.Organization{
			{
				ID: "a",
				OpenProject: config.OpenProjectCreds{
					URL:       "https://op.example.com",
					APIKeyEnv: "K",
				},
				Parent: config.ParentProject{Identifier: "a", Name: "A"},
				Products: []config.Product{
					{Identifier: "one", Name: "One", GitHub: "acme/core"},
					{Identifier: "two", Name: "Two", GitHub: "Acme/Core"},
				},
			},
		},
	}
	if err := r.Validate(); err == nil {
		t.Fatal("expected duplicate github error")
	}
}

func TestValidateDuplicateIdentifierSameInstance(t *testing.T) {
	t.Parallel()
	r := &config.Root{
		Organizations: []config.Organization{
			{
				ID: "acme",
				OpenProject: config.OpenProjectCreds{
					URL:       "https://op.example.com/",
					APIKeyEnv: "K1",
				},
				Parent:   config.ParentProject{Identifier: "acme", Name: "Acme"},
				Products: []config.Product{{Identifier: "core", Name: "Core", GitHub: "acme/core"}},
			},
			{
				ID: "globex",
				OpenProject: config.OpenProjectCreds{
					URL:       "https://op.example.com",
					APIKeyEnv: "K2",
				},
				Parent:   config.ParentProject{Identifier: "globex", Name: "Globex"},
				Products: []config.Product{{Identifier: "core", Name: "Core", GitHub: "globex/core"}},
			},
		},
	}
	if err := r.Validate(); err == nil {
		t.Fatal("expected duplicate identifier on same OP instance")
	}
}

func TestNormalizeRepoExpandsGitHub(t *testing.T) {
	t.Parallel()
	r := &config.Root{
		Organizations: []config.Organization{{
			ID:          "acme",
			OpenProject: config.OpenProjectCreds{URL: "https://op.example.com", APIKeyEnv: "K"},
			GitHub:      config.GitHubCreds{Org: "acme"},
			Parent:      config.ParentProject{Identifier: "acme", Name: "Acme"},
			Products: []config.Product{
				{ID: "core", Repo: "core"},
				{ID: "acme", Identifier: "acme-app", Repo: "acme"},
			},
		}},
	}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	if r.Organizations[0].Products[0].GitHub != "acme/core" {
		t.Fatalf("core github = %q", r.Organizations[0].Products[0].GitHub)
	}
	if r.Organizations[0].Products[1].Identifier != "acme-app" {
		t.Fatalf("identifier = %q", r.Organizations[0].Products[1].Identifier)
	}
}

func TestValidateProductIdentifierClashWithParent(t *testing.T) {
	t.Parallel()
	r := &config.Root{
		Organizations: []config.Organization{{
			ID:          "acme",
			OpenProject: config.OpenProjectCreds{URL: "https://op.example.com", APIKeyEnv: "K"},
			Parent:      config.ParentProject{Identifier: "acme", Name: "Acme"},
			Products:    []config.Product{{ID: "acme", GitHub: "acme/acme"}},
		}},
	}
	if err := r.Validate(); err == nil {
		t.Fatal("expected parent/product identifier clash")
	}
}

func TestResolveAPIKey(t *testing.T) {
	t.Setenv("OPENPROJECT_TEST_KEY", "tok-123")
	org := &config.Organization{
		ID: "t",
		OpenProject: config.OpenProjectCreds{
			APIKeyEnv: "OPENPROJECT_TEST_KEY",
		},
	}
	got, err := org.ResolveAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if got != "tok-123" {
		t.Fatalf("got %q", got)
	}
	_ = os.Unsetenv("OPENPROJECT_TEST_KEY")
	if _, err := org.ResolveAPIKey(); err == nil {
		t.Fatal("expected empty env error")
	}
}
