package opclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ErrNotFound means the resource does not exist.
var ErrNotFound = errors.New("openproject: not found")

// Client talks to OpenProject API v3 (Basic apikey:KEY).
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New creates an API client.
func New(baseURL, apiKey string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// BaseURL returns the configured instance URL.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// Project is a compact project from APIv3.
type Project struct {
	ID         int    `json:"id"`
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
	Active     bool   `json:"active"`
}

type collection[T any] struct {
	Total    int `json:"total"`
	Count    int `json:"count"`
	Embedded struct {
		Elements []T `json:"elements"`
	} `json:"_embedded"`
}

// Formattable is OpenProject's text field shape.
type Formattable struct {
	Format string `json:"format"`
	Raw    string `json:"raw"`
}

// CreateProjectInput is POST /api/v3/projects body.
type CreateProjectInput struct {
	Name        string
	Identifier  string
	Description string
	Public      bool
	ParentID    int // 0 = top-level
}

// GetByIdentifier looks up a project by identifier (or numeric id string).
func (c *Client) GetByIdentifier(ctx context.Context, identifier string) (*Project, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, fmt.Errorf("opclient: empty identifier")
	}

	var p Project
	status, err := c.doJSON(ctx, http.MethodGet, "/api/v3/projects/"+url.PathEscape(identifier), nil, &p)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, ErrNotFound
	}
	return &p, nil
}

// CreateProject creates a project; parent optional.
func (c *Client) CreateProject(ctx context.Context, in CreateProjectInput) (*Project, error) {
	body := map[string]any{
		"name":       in.Name,
		"identifier": in.Identifier,
		"public":     in.Public,
	}
	if strings.TrimSpace(in.Description) != "" {
		body["description"] = Formattable{Format: "markdown", Raw: in.Description}
	}
	if in.ParentID > 0 {
		body["_links"] = map[string]any{
			"parent": map[string]string{
				"href": fmt.Sprintf("/api/v3/projects/%d", in.ParentID),
			},
		}
	}

	var p Project
	status, err := c.doJSON(ctx, http.MethodPost, "/api/v3/projects", body, &p)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, ErrNotFound
	}
	return &p, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, payload any, dest any) (int, error) {
	var rdr io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return 0, fmt.Errorf("opclient: marshal: %w", err)
		}
		rdr = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return 0, fmt.Errorf("opclient: build request: %w", err)
	}
	req.SetBasicAuth("apikey", c.apiKey)
	req.Header.Set("Accept", "application/hal+json, application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("opclient: %s %s: %w", method, path, err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return res.StatusCode, fmt.Errorf("opclient: read body: %w", err)
	}

	if res.StatusCode == http.StatusNotFound {
		return res.StatusCode, nil
	}
	if res.StatusCode >= 400 {
		return res.StatusCode, fmt.Errorf("opclient: %s %s: HTTP %d: %s", method, path, res.StatusCode, trimErr(raw))
	}
	if dest != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, dest); err != nil {
			return res.StatusCode, fmt.Errorf("opclient: decode %s: %w", path, err)
		}
	}
	return res.StatusCode, nil
}

func trimErr(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}

// ProjectIDFromHref extracts a project id from a HAL href.
func ProjectIDFromHref(href string) int {
	parts := strings.Split(strings.Trim(href, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "projects" {
			id, _ := strconv.Atoi(parts[i+1])
			return id
		}
	}
	return 0
}
