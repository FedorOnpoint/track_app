# Go Middle Interview Tracker — план реализации

> Реализация выполняется последовательно по `tasks/todo.md`. Для каждого slice обязательны Red → Green → Refactor, относящиеся к изменению проверки и отдельное пользовательское разрешение на переход от плана к коду. Git commit, push, pull request и deployment запрещены без отдельной прямой команды пользователя.

## Цель

Создать production-ready личное веб-приложение для подготовки одного пользователя к собеседованию Middle Go Backend Developer: каталог знаний, вопросы и материалы, поиск, прогресс, checklist, повторение, закладки, заметки и безопасная поставка на Timeweb.

## Источник требований

- Утверждённая спецификация: `instructions.md`.
- Этот план уточняет порядок реализации, интерфейсы и проверки, но не расширяет scope спецификации.

## Зафиксированные решения

- Modular monolith: Go API на `net/http` + `chi/v5`, React SPA, PostgreSQL, Caddy.
- Go 1.27.1+, PostgreSQL 18.6, React 19.3, Vite 8.3; остальные версии фиксируются lock-файлами после проверки официальных источников непосредственно перед добавлением.
- Внутренние идентификаторы — UUID; URL тем используют стабильный `slug`.
- Миграции — `golang-migrate`; запросы — `pgx/v5` и `sqlc`.
- Единственный пользователь создаётся явной bootstrap-командой. Повторный bootstrap безопасен и не меняет существующий пароль.
- Серверное состояние frontend — TanStack Query; формы — React Hook Form + Zod.
- API использует `/api/v1`, JSON `snake_case`, RFC 3339 UTC и единый error envelope из спецификации.
- Заданные спецификацией `PUT` endpoints являются идемпотентной установкой полного состояния конкретного ресурса; заметки также обновляются через заданный `PUT`.
- UI: русский язык, адаптивный спокойный интерфейс, доступность WCAG 2.1 AA, light/dark theme без сложной анимации.
- Redis, broker, Kubernetes, GraphQL, SSR и регистрация пользователей исключены.

## Границы модулей

```text
platform/config ──┬──> platform/database ──> catalog ──> progress ──> dashboard
                 └──> platform/httpserver ─> auth ─────> notes/bookmarks

REST contract ──> generated/client types ──> frontend features/pages
catalog schema ─> versioned seed ──────────> search and learning UI
```

- `platform`: конфигурация, БД, HTTP lifecycle, request ID, безопасные ошибки и logging.
- `auth`: password hashing, bootstrap, sessions, CSRF/origin checks и login throttling.
- `catalog`: категории, темы, sections, вопросы, resources, фильтрация и пагинация.
- `progress`: статусы, review flag и checklist state.
- `bookmarks`, `notes`: только данные текущего authenticated user.
- `dashboard`: read model поверх catalog и пользовательских данных.
- Frontend организован по feature boundaries и не импортирует внутренности backend.

## API-контракт до реализации

### Общие правила

- Успешный одиночный ответ: `{ "data": { ... } }`; списки: `{ "data": [...], "pagination": { "page", "page_size", "total_items", "total_pages" } }`.
- Ошибка: `{ "error": { "code", "message", "fields"?, "request_id" } }`.
- Неизвестное JSON-поле, malformed JSON, неверный enum/UUID/slug, превышение body limit и некорректный query parameter дают контролируемый `4xx`.
- `401` одинаков для отсутствующей, истёкшей и отозванной сессии. Login не раскрывает существование пользователя.
- `GET` безопасны; заданные `PUT` операции идемпотентны; `POST /topics/{slug}/notes` не ретраится автоматически клиентом.
- `GET /topics` имеет детерминированный secondary sort по `slug`; `page >= 1`, `page_size` по умолчанию 20 и максимум 100.
- `query` ищет по title, summary, sections, questions и checklist text средствами PostgreSQL; внешнего search service нет.

### Основные payloads

- `POST /auth/login`: `{login,password}` → authenticated user; устанавливает opaque session cookie.
- `PUT /auth/password`: `{current_password,new_password}` → `204`; ротирует текущую сессию и отзывает остальные.
- `PUT /topics/{slug}/progress`: `{status,review_required}` → текущее состояние progress.
- `PUT /topics/{slug}/checklist/{item_id}`: `{completed}` → состояние checklist item.
- `PUT /topics/{slug}/bookmark` и `DELETE .../bookmark`: идемпотентны; возвращают `204`.
- `POST /topics/{slug}/notes`: `{body_markdown}` → `201` + note.
- `PUT /notes/{id}`: `{body_markdown}` → note; `DELETE /notes/{id}` → `204`.
- `GET /health/live` не обращается к БД; `GET /health/ready` выполняет bounded database ping.

Точные Go/TypeScript DTO фиксируются в slice, который первым их потребляет, и проверяются contract tests. OpenAPI не добавляется в MVP: это отдельный источник истины без указанного генерационного workflow.

## Структура файлов

```text
backend/cmd/{api,bootstrap,seed}/       entrypoints
backend/internal/platform/             config, database, HTTP server, observability
backend/internal/{auth,catalog,progress,bookmarks,notes,dashboard}/
backend/migrations/                    reversible SQL migrations
backend/queries/                       sqlc queries
backend/seeds/                         versioned Russian content
frontend/src/{app,pages,features,entities,shared}/
deploy/                                Compose, Caddy, backup/restore
.github/workflows/                     CI and gated deployment
tasks/                                 approved plan and execution state
```

Tests размещаются рядом с Go packages и frontend modules; Playwright smoke — в `frontend/e2e/`.

## Порядок vertical slices

### 1. Воспроизводимый каркас

1. Toolchain manifests, базовые backend/frontend builds и единый Makefile.
2. Development Compose с PostgreSQL, config validation и безопасными defaults.
3. HTTP lifecycle: timeouts, request ID, JSON errors, graceful shutdown, live/ready.

Checkpoint: чистый checkout собирается; `make test`, `make lint`, `make build` существуют и fail-fast.

### 2. Каталог знаний

4. Первая migration с catalog schema, constraints и индексами.
5. `sqlc` catalog queries и маленький идемпотентный test seed.
6. Categories и paginated topics API с валидацией всех фильтров.
7. Topic detail API со sections, checklist, questions, follow-ups, pitfalls, resources и practice.

Checkpoint: integration tests поднимают PostgreSQL через Testcontainers; повторный test seed не создаёт дублей.

### 3. Безопасная аутентификация

8. Users/sessions schema и bootstrap command.
9. Argon2id password service и opaque session token storage.
10. Login/logout/me middleware flow с generic errors и rate limiting.
11. Logout-all, password change, rotation, expiry и revocation.
12. Origin/Referer policy, body limits, cookie attributes и auth security review.

Checkpoint: приватный catalog закрыт; auth test matrix и race tests проходят.

### 4. Пользовательские данные

13. Progress/checklist schema и update API, включая concurrent update tests.
14. Bookmarks schema и list/set/delete API.
15. Notes schema и ownership-safe CRUD API.
16. Dashboard aggregates и last-opened behavior.
17. Review и track-list read models.

Checkpoint: restart PostgreSQL/API не теряет данные; пользовательские данные изолированы user_id predicates.

### 5. Основной frontend flow

18. App shell, router, typed API client, QueryClient и global error/401 behavior.
19. Login page и protected routing.
20. Dashboard с progress aggregates и продолжением последней темы.
21. Knowledge Map: URL-synced search, filters, pagination и empty states.
22. Topic page: theory, navigation, questions, resources и safe Markdown.
23. Topic interactions: status, review, checklist, bookmark и rollback-safe mutations.
24. Notes CRUD с accessible forms и delete confirmation.

Checkpoint: component tests покрывают loading/error/empty/success; основной flow работает с реальным API.

### 6. Дополнительные страницы и UX

25. Track List.
26. Bookmarks и Review lists.
27. Settings: password change и logout-all.
28. Responsive/a11y pass, keyboard navigation, focus, contrast и reduced motion.

Checkpoint: критические desktop/mobile flows доступны с клавиатуры и проходят automated accessibility smoke checks.

### 7. Полный seed-контент

29. Контентный schema validator и link-check workflow.
30–42. По одной законченной категории на каждый из 13 разделов спецификации.
43. Cross-category editorial review, 80–120 topics, duplicate/link/idempotency checks.

Checkpoint: каждая принятая категория полностью заполнена; seed update сохраняет progress, notes и bookmarks.

### 8. Production packaging и automation

44. Multi-stage non-root API/frontend images и production Caddy image/config.
45. Production Compose с internal network, healthchecks, immutable image refs и one-shot jobs.
46. Backup + retention + restore-check с адаптером выбранного S3-compatible storage.
47. CI: format, vet, staticcheck, race/unit/integration, frontend checks, sqlc diff, image build и scans.
48. CD dry-run: GHCR SHA tags, backup, migration gate, health/smoke и application rollback.
49. Playwright critical smoke flow и полный local production smoke.
50. Финальный security/quality review и acceptance matrix.

Реальный deployment не входит в автоматическое выполнение плана и требует отдельного разрешения и параметров из раздела 13 спецификации.

## Общая проверка каждого slice

1. Зафиксировать interfaces и написать минимальный failing test, если slice содержит поведение.
2. Запустить test и сохранить фактический Red результат.
3. Реализовать минимальный код до Green.
4. Запустить focused tests, format/lint/typecheck и относящийся build.
5. Проверить diff на secrets, подавления проверок и unrelated changes.
6. Обновить `tasks/todo.md` и отчитаться пользователю; не создавать commit.

## Основные риски и меры

| Риск | Уровень | Мера |
|---|---:|---|
| 80–120 тем снижают качество и раздувают review | высокий | выпускать по одной завершённой категории, schema validation и редакторский gate |
| Auth/CSRF/session ошибки | высокий | ранний отдельный slice, OWASP-проверка, integration/race tests, adversarial review |
| Seed уничтожает user state | высокий | разделить system/user tables, upsert только по stable keys, preservation tests до полного контента |
| Миграция проходит локально, но опасна в production | высокий | reversible migrations, pre-deploy backup, forward compatibility, без auto rollback данных |
| Поиск по нескольким типам контента становится медленным | средний | PostgreSQL FTS/trigram решение только после EXPLAIN на representative fixture |
| Vite 8/Rolldown ecosystem incompatibility | средний | проверить официальную compatibility до фиксации plugins; не добавлять лишние plugins |
| Testcontainers/Playwright делают CI нестабильным | средний | deterministic fixtures, readiness waits, bounded retries только на инфраструктурные события |
| Малый VPS испытывает resource pressure | средний | multi-stage images, pool/container limits, измерение production-like stack перед настройкой |

## Review focus

- Повторный запрос или конкурентное обновление не создаёт дубликаты и не теряет progress.
- Истёкшая/отозванная session cookie никогда не раскрывает приватные данные и очищается предсказуемо.
- Unicode/русский поиск, пустая строка, очень длинный query и комбинации фильтров дают стабильный результат.
- Markdown с raw HTML, scripts и опасными URL не исполняется.
- Частичный infrastructure failure (БД, migration, backup, healthcheck) останавливает deploy без удаления volume/backups.

## Отложенные решения, не блокирующие первые slices

- Домен, IP/SSH user и install path Timeweb.
- GitHub owner/repository и deploy key/user.
- Конкретный внешний S3-compatible backup provider и retention cost limits.
- Финальные Argon2id параметры выбираются по актуальной OWASP guidance на момент auth slice и фиксируются тестами/config.
- Точные resource limits выбираются после измерения production-like контейнеров.

## Gates

- Этот план должен быть отдельно одобрен до реализации.
- После каждого checkpoint требуется краткий отчёт и разрешение продолжить следующий крупный этап.
- Добавление dependency сверяется с официальной документацией и фиксируется lock-файлом.
- Auth, migrations, backup/restore и deployment проходят `doubt-driven-development` review.
- Перед готовностью этапа применяются code review и свежая verification evidence.
