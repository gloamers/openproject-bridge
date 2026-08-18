package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/gloamers/openproject-bridge/internal/defaults"

	"gopkg.in/yaml.v3"
)

// Root is the top-level ecosystem config.
type Root struct {
	Organizations []Organization `yaml:"organizations"`
	Bridge        Bridge         `yaml:"bridge"`
}

// Organization is one permission boundary (Jira-style project): OP parent + GitHub org.
// Several organizations MAY share the same OpenProject URL (one tracker, many parents).
// They MAY also use different URLs (isolated OP instances). Routing is
// repository.full_name → products[].github (owner/repo), often expanded from github.org + repo.
type Organization struct {
	ID          string           `yaml:"id"`
	Name        string           `yaml:"name"`
	OpenProject OpenProjectCreds `yaml:"openproject"`
	GitHub      GitHubCreds      `yaml:"github"`
	Parent      ParentProject    `yaml:"parent"`
	Defaults    OrgDefaults      `yaml:"defaults"`
	Products    []Product        `yaml:"products"`
	Projects    []Product        `yaml:"projects"` // deprecated alias for products
}

// OpenProjectCreds points at an OP instance; secrets via env names.
type OpenProjectCreds struct {
	URL         string `yaml:"url"`
	APIKeyEnv   string `yaml:"api_key_env"`
	UsernameEnv string `yaml:"username_env"`
	PasswordEnv string `yaml:"password_env"`
}

// GitHubCreds is the GitHub side of an organization (not a product).
type GitHubCreds struct {
	Org              string `yaml:"org"` // GitHub org/user login, e.g. acme
	WebhookSecretEnv string `yaml:"webhook_secret_env"`
}

// ParentProject is the root OP project for an org.
type ParentProject struct {
	Identifier  string `yaml:"identifier"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Public      bool   `yaml:"public"`
}

// OrgDefaults apply to products unless overridden.
type OrgDefaults struct {
	Sync      SyncDefaults      `yaml:"sync"`
	StatusMap map[string]string `yaml:"status_map"`
	TypeMap   map[string]string `yaml:"type_map"`
}

// SyncDefaults toggles inbound sync behavior.
type SyncDefaults struct {
	Issues     bool `yaml:"issues"`
	AttachSpec bool `yaml:"attach_spec"`
}

// Product is one line of work inside an organization (core, cli, …).
// OpenProject child project + optional GitHub repo.
type Product struct {
	ID          string        `yaml:"id"`
	Identifier  string        `yaml:"identifier"` // OP key; defaults to id (must differ from parent)
	Name        string        `yaml:"name"`
	Description string        `yaml:"description"`
	GitHub      string        `yaml:"github"` // owner/repo; or set repo: with github.org
	Repo        string        `yaml:"repo"`
	Sync        *SyncDefaults `yaml:"sync"`
}

// Bridge is shared webhook / mapping settings.
type Bridge struct {
	Webhook   WebhookConfig `yaml:"webhook"`
	MappingDB string        `yaml:"mapping_db"`
	Labels    Labels        `yaml:"labels"`
}

// WebhookConfig is the HTTP webhook listener.
type WebhookConfig struct {
	Listen       string `yaml:"listen"`
	Path         string `yaml:"path"`
	SecretEnv    string `yaml:"secret_env"`
	MaxBodyBytes int64  `yaml:"max_body_bytes"`
	AckMode      string `yaml:"ack_mode"`
}

// Labels are GitHub label names used by the bridge.
type Labels struct {
	Documented string `yaml:"documented"`
	Tracked    string `yaml:"tracked"`
}

// Load reads and validates YAML from path (secrets not resolved yet).
func Load(path string) (*Root, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	var root Root
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	if err := root.Normalize(); err != nil {
		return nil, err
	}
	if err := root.Validate(); err != nil {
		return nil, err
	}
	root.ApplyDefaults()
	return &root, nil
}

// Normalize expands aliases (projects → products, repo → github) in place.
func (r *Root) Normalize() error {
	for i := range r.Organizations {
		if err := r.Organizations[i].normalize(); err != nil {
			return err
		}
	}
	return nil
}

func (o *Organization) normalize() error {
	if len(o.Products) > 0 && len(o.Projects) > 0 {
		return fmt.Errorf("config: org %q: use products: or projects: (deprecated), not both", o.ID)
	}
	if len(o.Products) == 0 {
		o.Products = o.Projects
	}
	o.Projects = nil
	for i := range o.Products {
		if err := o.Products[i].normalize(o); err != nil {
			return fmt.Errorf("config: org %q: %w", o.ID, err)
		}
	}
	return nil
}

func (p *Product) normalize(org *Organization) error {
	if p.ID == "" {
		p.ID = p.Identifier
	}
	if p.Identifier == "" {
		p.Identifier = p.ID
	}
	if p.Name == "" {
		p.Name = p.ID
	}
	repo := strings.Trim(strings.TrimSpace(p.Repo), "/")
	if p.GitHub == "" && repo != "" {
		orgLogin := strings.TrimSpace(org.GitHub.Org)
		if orgLogin == "" {
			return fmt.Errorf("product %q: repo %q requires github.org on the organization", p.ID, repo)
		}
		p.GitHub = orgLogin + "/" + repo
	}
	gh := strings.TrimSpace(p.GitHub)
	if gh != "" && strings.TrimSpace(org.GitHub.Org) != "" {
		owner, _, ok := strings.Cut(gh, "/")
		if ok && !strings.EqualFold(owner, strings.TrimSpace(org.GitHub.Org)) {
			return fmt.Errorf("product %q: github %q is not in github.org %q", p.ID, gh, org.GitHub.Org)
		}
	}
	p.GitHub = gh
	return nil
}

// ApplyDefaults fills empty bridge fields with product defaults.
func (r *Root) ApplyDefaults() {
	if r.Bridge.Labels.Documented == "" {
		r.Bridge.Labels.Documented = defaults.LabelDocumented
	}
	if r.Bridge.Webhook.Listen == "" {
		r.Bridge.Webhook.Listen = defaults.WebhookListen
	}
	if r.Bridge.Webhook.Path == "" {
		r.Bridge.Webhook.Path = defaults.WebhookPath
	}
	if r.Bridge.Webhook.MaxBodyBytes <= 0 {
		r.Bridge.Webhook.MaxBodyBytes = defaults.MaxBodyBytes
	}
	if r.Bridge.MappingDB == "" {
		r.Bridge.MappingDB = defaults.MappingDB
	}
}

// DocumentedLabel returns the configured documented label (never empty after ApplyDefaults).
func (r *Root) DocumentedLabel() string {
	if r == nil || r.Bridge.Labels.Documented == "" {
		return defaults.LabelDocumented
	}
	return r.Bridge.Labels.Documented
}

// MappedStatus resolves GitHub lifecycle key (opened/closed) via status_map, with defaults.
func (o *Organization) MappedStatus(key string) string {
	if o != nil && o.Defaults.StatusMap != nil {
		if v := o.Defaults.StatusMap[key]; v != "" {
			return v
		}
	}
	switch key {
	case defaults.StatusKeyOpened:
		return defaults.OPStatusOpened
	case defaults.StatusKeyClosed:
		return defaults.OPStatusClosed
	default:
		return ""
	}
}

// Validate checks structural rules (no network / env).
func (r *Root) Validate() error {
	if err := r.Normalize(); err != nil {
		return err
	}
	if len(r.Organizations) == 0 {
		return fmt.Errorf("config: organizations is empty")
	}
	seenOrg := map[string]struct{}{}
	seenGitHub := map[string]struct{}{}
	seenIdent := map[string]struct{}{}               // orgID/identifier
	seenOnInstance := map[string]map[string]string{} // opURL → identifier → orgID

	claimInstanceIdent := func(orgID, opURL, ident string) error {
		key := instanceKey(opURL)
		if seenOnInstance[key] == nil {
			seenOnInstance[key] = map[string]string{}
		}
		if owner, ok := seenOnInstance[key][ident]; ok {
			return fmt.Errorf("config: OpenProject identifier %q is used by org %q and %q (same instance %s; identifiers are global like Jira keys)", ident, owner, orgID, key)
		}
		seenOnInstance[key][ident] = orgID
		return nil
	}

	for i, org := range r.Organizations {
		if org.ID == "" {
			return fmt.Errorf("config: organizations[%d]: id is required", i)
		}
		if _, ok := seenOrg[org.ID]; ok {
			return fmt.Errorf("config: duplicate organization id %q", org.ID)
		}
		seenOrg[org.ID] = struct{}{}

		if org.OpenProject.URL == "" {
			return fmt.Errorf("config: org %q: openproject.url is required", org.ID)
		}
		if org.OpenProject.APIKeyEnv == "" {
			return fmt.Errorf("config: org %q: openproject.api_key_env is required", org.ID)
		}
		if org.Parent.Identifier == "" || org.Parent.Name == "" {
			return fmt.Errorf("config: org %q: parent.identifier and parent.name are required", org.ID)
		}
		if !validOPIdentifier(org.Parent.Identifier) {
			return fmt.Errorf("config: org %q: parent.identifier %q is invalid", org.ID, org.Parent.Identifier)
		}
		if err := claimInstanceIdent(org.ID, org.OpenProject.URL, org.Parent.Identifier); err != nil {
			return err
		}

		parentKey := org.ID + "/" + org.Parent.Identifier
		seenIdent[parentKey] = struct{}{}
		seenProductID := map[string]struct{}{}

		for j, p := range org.Products {
			if p.ID == "" && p.Identifier == "" {
				return fmt.Errorf("config: org %q products[%d]: id (or identifier) is required", org.ID, j)
			}
			if p.Identifier == "" || p.Name == "" {
				return fmt.Errorf("config: org %q products[%d]: identifier and name are required", org.ID, j)
			}
			if !validOPIdentifier(p.Identifier) {
				return fmt.Errorf("config: org %q: product identifier %q is invalid", org.ID, p.Identifier)
			}
			if _, ok := seenProductID[p.ID]; ok {
				return fmt.Errorf("config: org %q: duplicate product id %q", org.ID, p.ID)
			}
			seenProductID[p.ID] = struct{}{}
			if err := claimInstanceIdent(org.ID, org.OpenProject.URL, p.Identifier); err != nil {
				return err
			}
			key := org.ID + "/" + p.Identifier
			if _, ok := seenIdent[key]; ok {
				return fmt.Errorf("config: org %q: duplicate product identifier %q", org.ID, p.Identifier)
			}
			seenIdent[key] = struct{}{}

			gh := strings.TrimSpace(p.GitHub)
			if gh == "" {
				continue
			}
			if !validGitHubRepo(gh) {
				return fmt.Errorf("config: org %q product %q: github %q must be owner/repo", org.ID, p.ID, gh)
			}
			low := strings.ToLower(gh)
			if _, ok := seenGitHub[low]; ok {
				return fmt.Errorf("config: duplicate github mapping %q", gh)
			}
			seenGitHub[low] = struct{}{}
		}
	}
	return nil
}

// OrgByID returns an organization or an error.
func (r *Root) OrgByID(id string) (*Organization, error) {
	for i := range r.Organizations {
		if r.Organizations[i].ID == id {
			return &r.Organizations[i], nil
		}
	}
	return nil, fmt.Errorf("config: organization %q not found", id)
}

// EffectiveSync merges product sync over org defaults.
func (o *Organization) EffectiveSync(p Product) SyncDefaults {
	out := o.Defaults.Sync
	if p.Sync != nil {
		out = *p.Sync
	}
	return out
}

func validOPIdentifier(s string) bool {
	if s == "" || len(s) > 100 {
		return false
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	return true
}

func validGitHubRepo(s string) bool {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return false
	}
	return parts[0] != "" && parts[1] != ""
}

func instanceKey(url string) string {
	s := strings.ToLower(strings.TrimSpace(url))
	s = strings.TrimRight(s, "/")
	return s
}
