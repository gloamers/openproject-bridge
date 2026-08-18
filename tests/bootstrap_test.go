//go:build integration

package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gloamers/openproject-bridge/internal/bootstrap"
	"github.com/gloamers/openproject-bridge/internal/config"
	"github.com/gloamers/openproject-bridge/internal/opclient"
)

func TestIntegration_BootstrapIdempotent(t *testing.T) {
	baseURL, apiKey := resolveItestOpenProject(t)
	ctx := context.Background()
	client := opclient.New(baseURL, apiKey, 60*time.Second)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000)
	org := &config.Organization{
		ID: "itest",
		OpenProject: config.OpenProjectCreds{
			URL: baseURL,
		},
		Parent: config.ParentProject{
			Identifier:  "br-parent-" + suffix,
			Name:        "Bridge Parent " + suffix,
			Description: "integration parent",
			Public:      false,
		},
		Products: []config.Product{
			{
				Identifier:  "br-core-" + suffix,
				Name:        "Bridge Core " + suffix,
				Description: "product",
				GitHub:      "itest/core-" + suffix,
			},
		},
	}

	rows1, err := bootstrap.EnsureOrganization(ctx, client, org, bootstrap.Options{})
	if err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}
	if len(rows1) != 2 {
		t.Fatalf("rows1=%d", len(rows1))
	}
	for _, r := range rows1 {
		if r.Status != bootstrap.StatusCreated {
			t.Fatalf("expected created, got %#v", r)
		}
		if r.OPID <= 0 {
			t.Fatalf("missing op id: %#v", r)
		}
	}

	rows2, err := bootstrap.EnsureOrganization(ctx, client, org, bootstrap.Options{})
	if err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	for _, r := range rows2 {
		if r.Status != bootstrap.StatusExists {
			t.Fatalf("expected exists, got %#v", r)
		}
	}
}
