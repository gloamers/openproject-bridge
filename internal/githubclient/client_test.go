package githubclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gloamers/openproject-bridge/internal/githubclient"
)

func TestCreateCommentAndLabel(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /repos/acme/core/issues/1/comments", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1})
	})
	mux.HandleFunc("POST /repos/acme/core/issues/1/labels", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]any{})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := &githubclient.Client{Token: "tok", BaseURL: srv.URL, HTTPClient: srv.Client()}
	if err := c.CreateComment(context.Background(), "acme", "core", 1, "hi"); err != nil {
		t.Fatal(err)
	}
	if err := c.AddLabel(context.Background(), "acme", "core", 1, "documented"); err != nil {
		t.Fatal(err)
	}
}
