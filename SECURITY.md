# Security Policy

## Supported versions

Security fixes land on `main` and apply to the latest release.

| Version | Supported          |
| ------- | ------------------ |
| main    | :white_check_mark: |

## Reporting a vulnerability

**Do not** open a public GitHub issue for security vulnerabilities.

1. **Private Security Advisory** (preferred):
   https://github.com/gloamers/openproject-bridge/security/advisories/new
2. If advisories are unavailable, email the maintainer listed in the GitHub org.

Include: description, reproduction, affected versions, impact.

### Response

- Initial response: within 72 hours
- Fix and disclosure: coordinated with the reporter

## What this project handles

- GitHub webhook HMAC (`X-Hub-Signature-256`); unsigned bodies are rejected
- Secrets via environment / `*_FILE` (absolute paths), not YAML literals
- SQLite mapping DB created with mode `0600`
- Per-org webhook secrets fail closed (no silent fallback to the bridge secret)

## What operators must do

- Put the webhook listener behind TLS (reverse proxy)
- Restrict who can reach `/webhooks/github`
- Rotate `GITHUB_WEBHOOK_SECRET` / OpenProject API keys independently
- Do not commit `configs/ecosystem.yaml` with live URLs and env names that leak tenant layout if that is sensitive — examples are generic

## Contact

- Advisories: https://github.com/gloamers/openproject-bridge/security/advisories/new
- Public bugs: https://github.com/gloamers/openproject-bridge/issues
