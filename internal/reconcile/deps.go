package reconcile

import (
	"context"

	"github.com/gloamers/openproject-bridge/internal/config"
	"github.com/gloamers/openproject-bridge/internal/opclient"
	"github.com/gloamers/openproject-bridge/internal/storage"
)

// IssueRepository lists persisted issue mappings.
type IssueRepository interface {
	ListIssues(ctx context.Context) ([]storage.IssueMapping, error)
}

// GitHubIssues reads issue state from GitHub.
type GitHubIssues interface {
	GetIssue(ctx context.Context, owner, repo string, number int) (state string, err error)
}

// OpenProject updates work package status during reconcile.
type OpenProject interface {
	FindStatusByName(ctx context.Context, name string) (*opclient.NamedResource, error)
	SetWorkPackageStatus(ctx context.Context, wpID, statusID int) (*opclient.WorkPackage, error)
}

// ClientFactory builds an OpenProject client for an organization.
type ClientFactory func(org *config.Organization) (OpenProject, error)
