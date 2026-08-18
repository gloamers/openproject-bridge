package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gloamers/openproject-bridge/internal/config"
)

func TestResolveSecretFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	if err := os.WriteFile(path, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MY_SECRET", "")
	t.Setenv("MY_SECRET_FILE", abs)
	got, err := config.ResolveSecret("MY_SECRET")
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-file" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveGitHub(t *testing.T) {
	t.Parallel()
	root := &config.Root{
		Organizations: []config.Organization{{
			ID:          "acme",
			OpenProject: config.OpenProjectCreds{URL: "https://op.example.com", APIKeyEnv: "K"},
			Parent:      config.ParentProject{Identifier: "acme", Name: "Acme"},
			Products:    []config.Product{{Identifier: "core", Name: "Core", GitHub: "Acme/Core"}},
		}},
	}
	if err := root.Validate(); err != nil {
		t.Fatal(err)
	}
	r, err := root.ResolveGitHub("acme/core")
	if err != nil || r.Product.Identifier != "core" {
		t.Fatalf("%#v %v", r, err)
	}
}
