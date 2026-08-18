# Contributing to OpenProject Bridge

Thank you for your interest in contributing.

## Requirements

- **Go 1.26+** (see `go.mod`)
- **[Task](https://taskfile.dev/)** — `task check:quick` / `task check`
- **golangci-lint** for full checks
- **Docker** only for `task test:integration`

## Quick start

```bash
git clone https://github.com/gloamers/openproject-bridge
cd openproject-bridge

task build
task test
golangci-lint run --timeout=5m ./cmd/... ./internal/...
```

## Workflow

1. Fork and clone
2. Branch: `feat/...` or `fix/...`
3. Add tests for behavior changes
4. Validate:

```bash
task check:quick              # fmt, vet, build, unit
task check:no-integration     # + race, lint
```

5. Open a pull request against `main` (GitHub Flow)

## Commit style

Prefer conventional commits:

- `feat:` new capability
- `fix:` bug fix
- `test:` tests only
- `docs:` documentation
- `chore:` tooling / CI

Author: `lkmavi <zikmanv@icloud.com>` unless specified otherwise.
Do not add `Co-authored-by:` trailers.

## Layout

| Path | Role |
|------|------|
| `cmd/` | Thin `main` for `op-bridge`, `op-bootstrap`, `op-document` |
| `internal/app/` | Builders + CLI (`Main`) |
| `internal/sync`, `webhook`, `knowledge`, `reconcile`, `bootstrap` | Services |
| `internal/storage` | Persistence port; `storage/sqlite` is the driver |
| `internal/defaults` | Shared product defaults |
| `tests/` | Docker OpenProject integration (`//go:build integration`) |

Interfaces live next to **consumers**, not next to implementations.

## Code of Conduct

See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
