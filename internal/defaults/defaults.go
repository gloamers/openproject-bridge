// Package defaults holds shared product defaults used by CLIs and services.
// Prefer config YAML / flags for overrides; these are fallbacks only.
package defaults

import "time"

// Paths and HTTP surface.
const (
	ConfigPath         = "configs/ecosystem.yaml"
	ADRDir             = "docs/adr"
	MappingDB          = "./data/mappings.sqlite"
	WebhookListen      = ":8443"
	WebhookPath        = "/webhooks/github"
	SpecAttachment     = "spec.md"
	SpecAttachmentDesc = "Issue body snapshot at open"
)

// Labels and OpenProject status names (when status_map omits a key).
const (
	LabelDocumented = "documented"
	OPStatusOpened  = "New"
	OPStatusClosed  = "Closed"
)

// Status map keys (GitHub issue lifecycle → status_map).
const (
	StatusKeyOpened = "opened"
	StatusKeyClosed = "closed"
)

// GitHub event / action names we handle.
const (
	EventIssues       = "issues"
	ActionOpened      = "opened"
	ActionClosed      = "closed"
	ActionReopened    = "reopened"
	ActionLabeled     = "labeled"
	GitHubStateClosed = "closed"
)

// Timeouts and worker tuning.
const (
	ShutdownTimeout = 15 * time.Second
	OPHTTPTimeout   = 30 * time.Second
	WebhookWorkers  = 2
	WebhookQueue    = 64
	MaxBodyBytes    = 1 << 20 // 1 MiB
	SQLiteBusyMs    = 5000
)

// Env secret names (values come from the environment, not from these constants).
const (
	EnvGitHubToken = "GITHUB_TOKEN" //nolint:gosec // G101: env var name, not a credential
)
