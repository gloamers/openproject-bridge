package webhook

import (
	"context"

	"github.com/gloamers/openproject-bridge/internal/domain"
)

// DeliveryRepository tracks webhook delivery idempotency.
type DeliveryRepository interface {
	BeginDelivery(ctx context.Context, deliveryID string) (started, alreadyDone bool, err error)
	CompleteDelivery(ctx context.Context, deliveryID string) error
	FailDelivery(ctx context.Context, deliveryID string) error
}

// IssueSync processes a verified delivery.
type IssueSync interface {
	Handle(ctx context.Context, d domain.Delivery) error
}
