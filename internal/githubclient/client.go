package githubclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a minimal GitHub REST client.
type Client struct {
	Token      string
	BaseURL    string // default https://api.github.com
	HTTPClient *http.Client
}

func (c *Client) base() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return "https://api.github.com"
}

func (c *Client) http() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// CreateComment posts a comment on an issue.
func (c *Client) CreateComment(ctx context.Context, owner, repo string, number int, body string) error {
	payload, _ := json.Marshal(map[string]string{"body": body})
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, number)
	return c.do(ctx, http.MethodPost, path, payload, nil)
}

// AddLabel adds a label to an issue.
func (c *Client) AddLabel(ctx context.Context, owner, repo string, number int, label string) error {
	payload, _ := json.Marshal([]string{label})
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/labels", owner, repo, number)
	return c.do(ctx, http.MethodPost, path, payload, nil)
}

// GetIssue fetches issue state (open/closed).
func (c *Client) GetIssue(ctx context.Context, owner, repo string, number int) (state string, err error) {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repo, number)
	var out struct {
		State string `json:"state"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return "", err
	}
	return out.State, nil
}

func (c *Client) do(ctx context.Context, method, path string, body []byte, dest any) error {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base()+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if res.StatusCode >= 400 {
		return fmt.Errorf("github: %s %s: HTTP %d: %s", method, path, res.StatusCode, strings.TrimSpace(string(raw)))
	}
	if dest != nil && len(raw) > 0 {
		return json.Unmarshal(raw, dest)
	}
	return nil
}
