# Go Middle Interview Tracker — execution checklist

Все задачи выполняются без Git commit/push/deploy, пока пользователь не даст отдельную прямую команду. Подробные решения, API contract, риски и gates находятся в `tasks/plan.md`.

## Этап 1 — воспроизводимый каркас

- [x] T01 Toolchain manifests, backend/frontend smoke builds и Makefile interface. Acceptance: `make test`, `make lint`, `make build` доступны без неописанных global tools. Verify: выполнить все три команды. Depends: —. Scope: M, до 5 файлов.
- [x] T02 Development PostgreSQL Compose и validated config. Acceptance: missing required config даёт понятную безопасную ошибку; БД не публикуется наружу. Verify: config unit tests + Compose config validation. Depends: T01. Scope: M.
- [x] T03 HTTP lifecycle и health endpoints. Acceptance: request ID/error envelope, server timeouts, graceful shutdown, live без БД, ready с bounded DB ping. Verify: `httptest`, integration readiness test, race test. Depends: T02. Scope: M.

### Checkpoint A

- [x] Чистая сборка backend/frontend, тесты и линтеры проходят; пользователь одобрил переход к каталогу.

## Этап 2 — каталог знаний

- [x] T04 Catalog migration. Acceptance: UUID keys, FK/check constraints, stable unique keys и индексы для заявленных lookups. Verify: migrate up/down/up на PostgreSQL 18.6. Depends: T02. Scope: M.
- [x] T05 sqlc catalog queries и идемпотентный test seed. Acceptance: повторный seed не создаёт дубликаты. Verify: integration test + `sqlc generate` leaves no diff. Depends: T04. Scope: M.
- [x] T06 Categories/list topics API. Acceptance: все фильтры валидируются, pagination capped, sort детерминирован. Verify: table/integration tests для valid и invalid combinations. Depends: T05,T03. Scope: M.
- [x] T07 Topic detail API. Acceptance: один payload содержит sections, checklist, questions, follow-ups, pitfalls, resources и practice; неизвестный slug → safe 404. Verify: contract integration tests. Depends: T05,T03. Scope: M.

### Checkpoint B

- [ ] Catalog API работает на PostgreSQL; migration/seed/filter tests и race suite проходят; пользователь одобрил auth.

## Этап 3 — аутентификация

- [x] T08 Users/sessions migration и bootstrap command. Acceptance: bootstrap idempotent, existing password не меняется, secrets не логируются. Verify: integration/CLI tests. Depends: T04. Scope: M.
- [x] T09 Password/session primitives. Acceptance: OWASP-aligned Argon2id, CSPRNG token ≥256 bits, only token hash stored, constant-shape credential failure. Verify: unit/property tests. Depends: T08. Scope: M.
- [x] T10 Login/logout/me и auth middleware. Acceptance: private endpoints return 401 without valid session; login rotates cookie; throttling bounded. Verify: `httptest` + PostgreSQL integration matrix. Depends: T09. Scope: M.
- [x] T11 Password change/logout-all/session lifecycle. Acceptance: expiry/revocation enforced; password change revokes other sessions and rotates current session. Verify: multi-session integration tests. Depends: T10. Scope: M.
- [x] T12 HTTP security boundary. Acceptance: Origin/Referer, cookie attributes, body limit and security headers behave by environment. Verify: table-driven security tests and manual header inspection. Depends: T10,T11. Scope: M.

### Checkpoint C

- [x] Auth threat review завершён; login/rate-limit/expiry/revocation/CSRF/race tests проходят; пользователь одобрил user data.

## Этап 4 — прогресс и личные данные

- [x] T13 Progress/checklist migration и API. Acceptance: allowed enums only, PUT idempotent, concurrent updates preserve valid state. Verify: integration + concurrency tests. Depends: T10,T07. Scope: M.
- [x] T14 Bookmarks migration и API. Acceptance: set/delete idempotent, list filtered to current user. Verify: integration tests. Depends: T10,T07. Scope: M.
- [x] T15 Notes migration и CRUD API. Acceptance: ownership enforced in SQL predicates; body bounded; 404 avoids existence disclosure. Verify: two-user integration tests. Depends: T10,T07. Scope: M.
- [x] T16 Dashboard aggregates и last-opened. Acceptance: empty/full category math correct, recent items deterministic. Verify: aggregate integration tests. Depends: T13,T15. Scope: M.
- [x] T17 Review и track-list read models. Acceptance: filters/search/pagination follow shared contract. Verify: integration contract tests. Depends: T13,T14. Scope: M.

### Checkpoint D

- [x] Persistent user flow доказан после restart; ownership, aggregate и concurrency tests проходят; пользователь одобрил frontend.

## Этап 5 — основной frontend flow

- [x] T18 App shell/router/API client/QueryClient. Acceptance: typed envelopes, abortable requests, global 401 and safe error mapping. Verify: unit/component tests + production build. Depends: T10,T17. Scope: M.
- [x] T19 Login и protected routes. Acceptance: accessible form, generic error, redirect after success/401. Verify: RTL tests. Depends: T18. Scope: M.
- [x] T20 Dashboard page. Acceptance: progress/category/review/recent/continue states render for loading/error/empty/success. Verify: RTL tests. Depends: T18,T16. Scope: M.
- [x] T21 Knowledge Map. Acceptance: search/filter/page state synchronized with URL and resettable. Verify: RTL router tests. Depends: T18,T06. Scope: M.
- [x] T22 Topic content page и safe Markdown. Acceptance: semantic navigation; raw HTML and unsafe links do not execute. Verify: component/security tests. Depends: T18,T07. Scope: M.
- [x] T23 Topic progress/checklist/bookmark mutations. Acceptance: double submit prevented; optimistic rollback restores exact previous state. Verify: mutation component tests. Depends: T22,T13,T14. Scope: M.
- [x] T24 Notes CRUD UI. Acceptance: create/edit/delete confirmation, keyboard access, rollback/error state. Verify: RTL tests. Depends: T22,T15. Scope: M.

### Checkpoint E

- [ ] Login → search → topic → progress/note/bookmark работает с реальным API; component/build checks проходят.

## Этап 6 — дополнительные страницы и accessibility

- [ ] T25 Track List page. Acceptance: compact responsive table/list, status control, no bulk change. Verify: RTL + responsive browser check. Depends: T18,T17. Scope: M.
- [ ] T26 Bookmarks и Review pages. Acceptance: independent search/filters and direct topic/status actions. Verify: RTL tests. Depends: T18,T17. Scope: M.
- [ ] T27 Settings page. Acceptance: password form verifies current password; logout-all returns to login. Verify: RTL + API integration flow. Depends: T18,T11. Scope: M.
- [ ] T28 Responsive/a11y pass. Acceptance: keyboard navigation, visible focus, semantic landmarks, AA contrast, reduced motion. Verify: automated accessibility + DevTools mobile/keyboard inspection. Depends: T19–T27. Scope: M.

### Checkpoint F

- [ ] Все MVP screens доступны на desktop/mobile; accessibility findings устранены или явно согласованы.

## Этап 7 — production seed

- [ ] T29 Seed schema validator и external-link checker. Acceptance: required fields/counts/stable keys validated; network failures distinguished from invalid URLs. Verify: fixture tests. Depends: T05. Scope: M.
- [ ] T30 Go Core category. Verify: schema/editorial/link/idempotency checks. Depends: T29. Scope: M.
- [ ] T31 Runtime и память category. Verify: те же checks. Depends: T29. Scope: M.
- [ ] T32 Конкурентность category. Verify: те же checks. Depends: T29. Scope: M.
- [ ] T33 Standard Library category. Verify: те же checks. Depends: T29. Scope: M.
- [ ] T34 HTTP и API category. Verify: те же checks. Depends: T29. Scope: M.
- [ ] T35 PostgreSQL и SQL category. Verify: те же checks. Depends: T29. Scope: M.
- [ ] T36 Архитектура category. Verify: те же checks. Depends: T29. Scope: M.
- [ ] T37 Тестирование category. Verify: те же checks. Depends: T29. Scope: M.
- [ ] T38 Инфраструктура category. Verify: те же checks. Depends: T29. Scope: M.
- [ ] T39 Production category. Verify: те же checks. Depends: T29. Scope: M.
- [ ] T40 Безопасность category. Verify: те же checks. Depends: T29. Scope: M.
- [ ] T41 System Design category. Verify: те же checks. Depends: T29. Scope: M.
- [ ] T42 Поведение на собеседовании category. Verify: те же checks. Depends: T29. Scope: M.
- [ ] T43 Cross-category editorial gate. Acceptance: 80–120 complete unique topics, valid primary-source links, no duplicated questions, user data preserved on reseed. Verify: full seed + preservation/link/report tests. Depends: T30–T42. Scope: M.

### Checkpoint G

- [ ] Полный seed принят пользователем по категориям; техническая редактура и preservation tests проходят.

## Этап 8 — production packaging и CI/CD

- [ ] T44 Production images/Caddy. Acceptance: multi-stage, non-root where supported, static frontend, `/api/*` proxy, required headers. Verify: image builds + container inspection. Depends: checkpoints E–G. Scope: M.
- [ ] T45 Production Compose/jobs. Acceptance: only Caddy publishes 80/443; DB/API internal; healthchecks, volumes, migrate/seed profiles. Verify: `docker compose config` + clean-stack smoke. Depends: T44. Scope: M.
- [ ] T46 Backup/retention/restore-check. Acceptance: custom-format dump, 7 daily/4 weekly, encrypted external upload adapter, restore into test DB. Verify: generated backup + restore data assertions. Depends: T45 and selected provider. Scope: M.
- [ ] T47 CI workflow. Acceptance: backend/frontend/sqlc/images/scans are gates and parallel where safe. Verify: local equivalents + workflow syntax/dry-run checks. Depends: all implementation tasks. Scope: M.
- [ ] T48 CD dry-run workflow. Acceptance: SHA images, least-privilege SSH, backup/migrate/health/smoke gates and app-tag rollback; no destructive DB rollback. Verify: mocked/dry-run job. Depends: T45–T47 and deployment parameters. Scope: M.
- [ ] T49 Playwright + local production smoke. Acceptance: exact critical flow from spec persists across reload. Verify: `make e2e`, `make smoke`. Depends: T43,T45. Scope: M.
- [ ] T50 Final review/acceptance matrix. Acceptance: all criteria mapped to fresh evidence; no secrets/open DB ports/suppressed checks. Verify: `make lint test build e2e smoke restore-check` plus security review. Depends: T46–T49. Scope: M.

### Checkpoint H

- [ ] Все acceptance criteria имеют свежие доказательства; реальный deploy ожидает отдельного разрешения и параметров Timeweb/GitHub/S3.
