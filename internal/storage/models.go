// Package storage holds shared persistence models for sync state
// (GitHub issue ↔ OpenProject work package). Drivers live in subpackages
// (e.g. storage/sqlite); interfaces are declared by consumers.
package storage

// IssueKey identifies a GitHub issue.
type IssueKey struct {
	Owner  string
	Repo   string
	Number int
}

// IssueMapping links a GitHub issue to an OpenProject work package.
type IssueMapping struct {
	IssueKey
	OrgID     string
	ProjectID int
	WPID      int
	WPURL     string
}
