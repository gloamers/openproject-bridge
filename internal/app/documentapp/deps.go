package documentapp

import (
	"context"

	"github.com/gloamers/openproject-bridge/internal/storage"
)

// IssueStore looks up GitHub issue ↔ work package mappings for ADR promotion.
// Declared here (consumer), not in storage/.
type IssueStore interface {
	GetIssue(ctx context.Context, k storage.IssueKey) (*storage.IssueMapping, error)
	Close() error
}

// OpenFunc opens an IssueStore for a path/DSN.
type OpenFunc func(path string) (IssueStore, error)
