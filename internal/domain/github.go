package domain

import "strings"

// Delivery is a verified GitHub webhook payload ready for sync.
type Delivery struct {
	Event      string
	DeliveryID string
	Body       []byte
}

// IssueEvent is the subset of a GitHub issues webhook payload we need.
type IssueEvent struct {
	Action string `json:"action"`
	Issue  struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
		State   string `json:"state"`
		Labels  []struct {
			Name string `json:"name"`
		} `json:"labels"`
	} `json:"issue"`
	Repository struct {
		FullName string `json:"full_name"`
		Name     string `json:"name"`
		Owner    struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
	Label *struct {
		Name string `json:"name"`
	} `json:"label"`
}

// RepoFullName returns owner/repo from the event.
func (e IssueEvent) RepoFullName() string {
	if e.Repository.FullName != "" {
		return e.Repository.FullName
	}
	return e.Repository.Owner.Login + "/" + e.Repository.Name
}

// OwnerRepo returns owner and repo name.
func (e IssueEvent) OwnerRepo() (owner, repo string) {
	return SplitRepo(e.Repository.FullName, e.Repository.Owner.Login, e.Repository.Name)
}

// SplitRepo parses owner/repo from a full name or falls back to parts.
func SplitRepo(full, owner, name string) (string, string) {
	if full != "" {
		parts := strings.SplitN(full, "/", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
	}
	return owner, name
}
