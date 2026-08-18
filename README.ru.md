<p align="center">
  <img src="assets/logo.svg" alt="OpenProject Bridge" width="128" />
</p>

<h1 align="center">OpenProject Bridge</h1>

<p align="center">
  <strong>GitHub ↔ OpenProject для мульти-орг экосистем</strong><br>
  Bootstrap проектов · webhook-синхронизация · ADR<br>
  Раздельные слои + ссылки — Go + OpenProject
</p>

<p align="center">
  <code>op-bootstrap</code> · <code>op-bridge</code> · <code>op-document</code>
</p>

<p align="center">
  <a href="README.md">English</a> ·
  <a href="README.ru.md"><b>Русский</b></a>
</p>

<p align="center">
  <a href="https://github.com/gloamers/openproject-bridge/actions/workflows/ci.yml"><img src="https://github.com/gloamers/openproject-bridge/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://codecov.io/gh/gloamers/openproject-bridge"><img src="https://codecov.io/gh/gloamers/openproject-bridge/branch/main/graph/badge.svg" alt="codecov"></a>
  <img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/OpenProject-API%20v3-blue" alt="OpenProject">
  <img src="https://img.shields.io/badge/sync-GitHub%20webhooks-success" alt="Webhooks">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License"></a>
</p>

---

## Обзор

**openproject-bridge** — конфиг-драйвен мост между [GitHub](https://github.com/) и [OpenProject](https://www.openproject.org/) на любое число организаций.

Модель: **discussion / tracker / knowledge** живут в разных системах и связаны **ссылками**, без зеркала body/комментариев. Дополняет (не заменяет) [официальную GitHub-интеграцию OpenProject](https://www.openproject.org/docs/system-admin-guide/integrations/github-integration/).

### Возможности

| Область | Что делает |
|---------|------------|
| **Bootstrap** | Multi-org YAML → идемпотентные parent + product проекты (`op-bootstrap`) |
| **Intake** | Webhook `issues` → work package + optional `spec.md` |
| **Статус** | close / reopen → статус OP через `status_map` |
| **Knowledge** | Label `documented` → ADR + комментарий в GitHub |
| **Идемпотентность** | HMAC-SHA256 + `delivery_id` в SQLite |
| **Секреты** | env / `*_FILE`; Keychain через `task run:bridge` |

---

## Требования

- **Go 1.26+**
- OpenProject API v3 (интеграционные тесты: `openproject/openproject:17`)

```bash
task build
```

---

## Быстрый старт

```bash
cp configs/ecosystem.example.yaml configs/ecosystem.yaml
task build
./bin/op-bootstrap -config configs/ecosystem.yaml -org acme -dry-run
./bin/op-bootstrap -config configs/ecosystem.yaml -org acme
./bin/op-bridge -config configs/ecosystem.yaml -listen :8443
```

Webhook: `https://bridge.example.com/webhooks/github`.

---

## CLI

| Бинарник | Роль |
|----------|------|
| `op-bootstrap` | Проекты OP из YAML (`-dry-run`, `-org`) |
| `op-bridge` | Webhook-сервер; `-reconcile` один проход |
| `op-document` | ADR для `owner/repo#n` |

---

## Разработка

Нужен [Task](https://taskfile.dev/).

```bash
task check:quick
task check:no-integration
task check
```

Секреты: `task run:bridge` / `task run:bootstrap` (macOS Keychain или `secret-tool`).

Подробнее: [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md).

---

## Лицензия

[MIT](LICENSE)

---

<p align="center">
  <strong>OpenProject Bridge</strong> — обсуждение в GitHub, трекинг в OpenProject, решения в git
</p>
