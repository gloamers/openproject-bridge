package knowledge

import "context"

// IssueNotifier posts comments and labels on GitHub issues.
type IssueNotifier interface {
	CreateComment(ctx context.Context, owner, repo string, number int, body string) error
	AddLabel(ctx context.Context, owner, repo string, number int, label string) error
}
