package opclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
)

// WorkPackage is a subset of APIv3 work package.
type WorkPackage struct {
	ID          int         `json:"id"`
	LockVersion int         `json:"lockVersion"`
	Subject     string      `json:"subject"`
	Description Formattable `json:"description"`
	Links       struct {
		Status  Link `json:"status"`
		Type    Link `json:"type"`
		Project Link `json:"project"`
		Self    Link `json:"self"`
	} `json:"_links"`
}

// Link is a HAL link.
type Link struct {
	Href  string `json:"href"`
	Title string `json:"title"`
}

// NamedResource is status/type with id+name.
type NamedResource struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// CreateWorkPackageInput for POST /api/v3/work_packages.
type CreateWorkPackageInput struct {
	ProjectID   int
	Subject     string
	Description string
	TypeID      int // 0 = default
	StatusID    int // 0 = default
}

// CreateWorkPackage creates a work package in a project.
func (c *Client) CreateWorkPackage(ctx context.Context, in CreateWorkPackageInput) (*WorkPackage, error) {
	body := map[string]any{
		"subject": in.Subject,
		"_links": map[string]any{
			"project": map[string]string{
				"href": fmt.Sprintf("/api/v3/projects/%d", in.ProjectID),
			},
		},
	}
	if strings.TrimSpace(in.Description) != "" {
		body["description"] = Formattable{Format: "markdown", Raw: in.Description}
	}
	links := body["_links"].(map[string]any)
	if in.TypeID > 0 {
		links["type"] = map[string]string{"href": fmt.Sprintf("/api/v3/types/%d", in.TypeID)}
	}
	if in.StatusID > 0 {
		links["status"] = map[string]string{"href": fmt.Sprintf("/api/v3/statuses/%d", in.StatusID)}
	}

	var wp WorkPackage
	_, err := c.doJSON(ctx, http.MethodPost, "/api/v3/work_packages", body, &wp)
	if err != nil {
		return nil, err
	}
	return &wp, nil
}

// GetWorkPackage fetches a work package by id.
func (c *Client) GetWorkPackage(ctx context.Context, id int) (*WorkPackage, error) {
	var wp WorkPackage
	status, err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/v3/work_packages/%d", id), nil, &wp)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, ErrNotFound
	}
	return &wp, nil
}

// SetWorkPackageStatus patches status (retries on lockVersion conflict).
func (c *Client) SetWorkPackageStatus(ctx context.Context, wpID, statusID int) (*WorkPackage, error) {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		cur, err := c.GetWorkPackage(ctx, wpID)
		if err != nil {
			return nil, err
		}
		body := map[string]any{
			"lockVersion": cur.LockVersion,
			"_links": map[string]any{
				"status": map[string]string{
					"href": fmt.Sprintf("/api/v3/statuses/%d", statusID),
				},
			},
		}
		var wp WorkPackage
		status, err := c.doJSON(ctx, http.MethodPatch, fmt.Sprintf("/api/v3/work_packages/%d", wpID), body, &wp)
		if err == nil {
			return &wp, nil
		}
		last = err
		if status == http.StatusConflict || status == http.StatusUnprocessableEntity {
			continue
		}
		return nil, err
	}
	return nil, last
}

// FindStatusByName returns the first status whose name matches (case-insensitive).
func (c *Client) FindStatusByName(ctx context.Context, name string) (*NamedResource, error) {
	return c.findNamed(ctx, "/api/v3/statuses", name)
}

// FindTypeByName returns the first type whose name matches (case-insensitive).
func (c *Client) FindTypeByName(ctx context.Context, name string) (*NamedResource, error) {
	return c.findNamed(ctx, "/api/v3/types", name)
}

func (c *Client) findNamed(ctx context.Context, path, name string) (*NamedResource, error) {
	want := strings.ToLower(strings.TrimSpace(name))
	q := url.Values{}
	q.Set("pageSize", "100")
	var out collection[NamedResource]
	_, err := c.doJSON(ctx, http.MethodGet, path+"?"+q.Encode(), nil, &out)
	if err != nil {
		return nil, err
	}
	for i := range out.Embedded.Elements {
		el := out.Embedded.Elements[i]
		if strings.ToLower(el.Name) == want {
			return &el, nil
		}
	}
	return nil, fmt.Errorf("opclient: %s named %q not found", path, name)
}

// AddAttachment uploads a file to a work package.
func (c *Client) AddAttachment(ctx context.Context, wpID int, filename string, content []byte, description string) error {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	meta := map[string]any{
		"fileName": filename,
	}
	if description != "" {
		meta["description"] = map[string]string{"format": "plain", "raw": description}
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="metadata"`)
	h.Set("Content-Type", "application/json")
	pw, err := w.CreatePart(h)
	if err != nil {
		return err
	}
	if _, err := pw.Write(metaJSON); err != nil {
		return err
	}

	fh, err := w.CreateFormFile("file", filename)
	if err != nil {
		return err
	}
	if _, err := fh.Write(content); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	path := fmt.Sprintf("/api/v3/work_packages/%d/attachments", wpID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, &buf)
	if err != nil {
		return err
	}
	req.SetBasicAuth("apikey", c.apiKey)
	req.Header.Set("Accept", "application/hal+json, application/json")
	req.Header.Set("Content-Type", w.FormDataContentType())

	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if res.StatusCode >= 400 {
		return fmt.Errorf("opclient: attach: HTTP %d: %s", res.StatusCode, trimErr(raw))
	}
	return nil
}

// WorkPackageURL builds a UI URL.
func (c *Client) WorkPackageURL(id int) string {
	return fmt.Sprintf("%s/work_packages/%d", c.baseURL, id)
}
