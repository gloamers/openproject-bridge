package opclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gloamers/openproject-bridge/internal/opclient"
)

func TestGetByIdentifierAndCreate(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v3/projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "apikey" || pass != "key-1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("POST /api/v3/projects", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["identifier"] != "acme" {
			t.Fatalf("identifier = %#v", body["identifier"])
		}
		links, _ := body["_links"].(map[string]any)
		if links != nil {
			t.Fatalf("unexpected parent links: %#v", links)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         10,
			"identifier": "acme",
			"name":       "Acme",
			"active":     true,
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := opclient.New(srv.URL, "key-1", 0)
	ctx := context.Background()

	_, err := c.GetByIdentifier(ctx, "acme")
	if !errors.Is(err, opclient.ErrNotFound) {
		t.Fatalf("GetByIdentifier: %v", err)
	}

	p, err := c.CreateProject(ctx, opclient.CreateProjectInput{
		Name:       "Acme",
		Identifier: "acme",
		Public:     false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != 10 || p.Identifier != "acme" {
		t.Fatalf("project = %#v", p)
	}
}

func TestCreateWithParent(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v3/projects", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		links := body["_links"].(map[string]any)
		parent := links["parent"].(map[string]any)
		if parent["href"] != "/api/v3/projects/10" {
			t.Fatalf("parent = %#v", parent)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         11,
			"identifier": "core",
			"name":       "Core",
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := opclient.New(srv.URL, "k", 0)
	p, err := c.CreateProject(context.Background(), opclient.CreateProjectInput{
		Name:       "Core",
		Identifier: "core",
		ParentID:   10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != 11 {
		t.Fatalf("id = %d", p.ID)
	}
}
