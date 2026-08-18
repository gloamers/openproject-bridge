package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GitHubRoute is a resolved repo → organization + product mapping.
type GitHubRoute struct {
	Org     *Organization
	Product Product
}

// ResolveGitHub finds org+product for owner/repo (case-insensitive).
func (r *Root) ResolveGitHub(fullName string) (*GitHubRoute, error) {
	want := strings.ToLower(strings.TrimSpace(fullName))
	for i := range r.Organizations {
		org := &r.Organizations[i]
		for _, p := range org.Products {
			if strings.ToLower(strings.TrimSpace(p.GitHub)) == want {
				return &GitHubRoute{Org: org, Product: p}, nil
			}
		}
	}
	return nil, fmt.Errorf("config: no product mapped to github %q", fullName)
}

// ResolveSecret reads env, or env+"_FILE" contents (trimmed).
func ResolveSecret(envName string) (string, error) {
	if envName == "" {
		return "", fmt.Errorf("config: empty secret env name")
	}
	if v := strings.TrimSpace(os.Getenv(envName)); v != "" {
		return v, nil
	}
	fileEnv := envName + "_FILE"
	path := strings.TrimSpace(os.Getenv(fileEnv))
	if path == "" {
		return "", fmt.Errorf("config: env %s (or %s) is empty", envName, fileEnv)
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("config: %s must be an absolute path", fileEnv)
	}
	b, err := os.ReadFile(path) //nolint:gosec // path from operator-controlled *_FILE env
	if err != nil {
		return "", fmt.Errorf("config: read %s: %w", path, err)
	}
	v := strings.TrimSpace(string(b))
	if v == "" {
		return "", fmt.Errorf("config: secret file %s is empty", path)
	}
	return v, nil
}

// ResolveAPIKey reads the API key for an org from env or *_FILE.
func (o *Organization) ResolveAPIKey() (string, error) {
	v, err := ResolveSecret(o.OpenProject.APIKeyEnv)
	if err != nil {
		return "", fmt.Errorf("config: org %q: %w", o.ID, err)
	}
	return v, nil
}

// ResolveWebhookSecret returns the org webhook secret.
// If the org defines github.webhook_secret_env, that secret is required (no silent fallback).
// Orgs without a dedicated secret use bridge.webhook.secret_env.
func (r *Root) ResolveWebhookSecret(org *Organization) (string, error) {
	if org != nil && org.GitHub.WebhookSecretEnv != "" {
		v, err := ResolveSecret(org.GitHub.WebhookSecretEnv)
		if err != nil {
			return "", fmt.Errorf("config: org %q webhook secret: %w", org.ID, err)
		}
		return v, nil
	}
	return r.FallbackWebhookSecret()
}

// FallbackWebhookSecret resolves only the bridge-level secret.
func (r *Root) FallbackWebhookSecret() (string, error) {
	if r.Bridge.Webhook.SecretEnv == "" {
		return "", fmt.Errorf("config: bridge.webhook.secret_env is empty")
	}
	return ResolveSecret(r.Bridge.Webhook.SecretEnv)
}
