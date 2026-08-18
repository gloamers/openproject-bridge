package sync

import (
	"context"

	"github.com/gloamers/openproject-bridge/internal/config"
	"github.com/gloamers/openproject-bridge/internal/domain"
	"github.com/gloamers/openproject-bridge/internal/opclient"
	"github.com/gloamers/openproject-bridge/internal/storage"
)

// OpenProject is the OpenProject surface used by the sync service.
// Declared here (consumer), not next to opclient.
type OpenProject interface {
	GetByIdentifier(ctx context.Context, identifier string) (*opclient.Project, error)
	CreateWorkPackage(ctx context.Context, in opclient.CreateWorkPackageInput) (*opclient.WorkPackage, error)
	SetWorkPackageStatus(ctx context.Context, wpID, statusID int) (*opclient.WorkPackage, error)
	FindStatusByName(ctx context.Context, name string) (*opclient.NamedResource, error)
	FindTypeByName(ctx context.Context, name string) (*opclient.NamedResource, error)
	AddAttachment(ctx context.Context, wpID int, filename string, content []byte, description string) error
	WorkPackageURL(id int) string
}

// ClientFactory builds an OpenProject client for an organization.
type ClientFactory func(org *config.Organization) (OpenProject, error)

// IssueRepository persists GitHub issue ↔ work package mappings.
type IssueRepository interface {
	GetIssue(ctx context.Context, k storage.IssueKey) (*storage.IssueMapping, error)
	UpsertIssue(ctx context.Context, m storage.IssueMapping) error
}

// Documenter is called when the documented label is applied.
type Documenter interface {
	OnDocumented(ctx context.Context, route *config.GitHubRoute, issue domain.IssueEvent, wpURL string) error
}
