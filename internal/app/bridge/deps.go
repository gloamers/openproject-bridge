package bridge

import (
	"context"

	"github.com/gloamers/openproject-bridge/internal/storage"
)

// Store is the persistence surface used by the bridge composition root.
// Declared here (consumer), not in storage/.
type Store interface {
	Close() error

	BeginDelivery(ctx context.Context, deliveryID string) (started, alreadyDone bool, err error)
	CompleteDelivery(ctx context.Context, deliveryID string) error
	FailDelivery(ctx context.Context, deliveryID string) error

	GetIssue(ctx context.Context, k storage.IssueKey) (*storage.IssueMapping, error)
	UpsertIssue(ctx context.Context, m storage.IssueMapping) error
	ListIssues(ctx context.Context) ([]storage.IssueMapping, error)
}

// OpenFunc opens a Store for a path/DSN (driver chosen by the composition root).
type OpenFunc func(path string) (Store, error)
