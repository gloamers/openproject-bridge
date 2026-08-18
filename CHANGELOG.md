# Changelog

All notable changes to this project are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Repository layout: services, consumer-side interfaces, `storage/sqlite`, app builders
- GitHub Flow CI (build/test/lint/format/deps + Docker integration)
- Taskfile-only ops (`task check`, `task run:bridge`, `task run:bootstrap`)

### Changed

- Persistence port lives in `internal/storage`; SQLite is a driver, not the package API
- Product defaults centralized in `internal/defaults`
- Example config uses generic org names (`acme` / `globex` / `client-x`) in a single `ecosystem.example.yaml`

[Unreleased]: https://github.com/gloamers/openproject-bridge/compare/HEAD...HEAD
