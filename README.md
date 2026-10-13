# Go Middle Interview Tracker

Личный каталог подготовки к Middle Go: поиск по материалам, вопросы, checklist, прогресс, повторение, закладки и заметки. Системный контент находится в `backend/seeds/*.v1.json`; пользовательские данные хранятся отдельно в PostgreSQL.

## Локальные проверки

Нужны Docker с Compose v2 (поддержка `!override`), Go toolchain, npm/npx и Python 3.9+. Makefile выбирает Go 1.27.1 и Node 22.22.0; дополнительные инструменты устанавливаются в игнорируемую `.tools`.

```sh
make tools
make lint test test-integration test-operations
make seed-catalog check-seed-links
make staticcheck generate-check workflow-check build
make docker-build security-scan
make smoke
```

`make smoke` создаёт отдельный случайно именованный локальный стек на `127.0.0.1:4180`, запускает migrations/seed/bootstrap, реальный Playwright flow, повторный seed и restart. Учётные данные генерируются внутри проверки. После проверки удаляются только её контейнеры и volumes. На macOS можно использовать установленный Chrome: `PLAYWRIGHT_CHANNEL=chrome make smoke`. Для Linux Playwright также требует системные browser dependencies (`npx playwright install --with-deps chromium`).

`make test-browser` проверяет desktop/mobile accessibility на Vite; `make e2e` проверяет уже запущенный API/frontend. Для второго нужны `E2E_BASE_URL`, `E2E_LOGIN`, `E2E_PASSWORD`; это должна быть отдельная тестовая учётная запись.

## Development

Скопируйте `.env.example` в игнорируемый `.env`, задайте собственный пароль PostgreSQL и соответствующий `DATABASE_URL`. `make dev` поднимает PostgreSQL во внутренней Docker-сети, `make stop` останавливает его без удаления volume. API и одноразовые jobs должны запускаться в той же сети; порт PostgreSQL наружу не публикуется.

`make migrate-up` применяет только migrations up; `make seed` загружает весь каталог по стабильным ключам. Эти команды читают конфигурацию из окружения процесса, поэтому для host-запуска понадобится доступный ему тестовый PostgreSQL. Перед seed всегда применяйте migrations. Bootstrap запускается отдельно с `BOOTSTRAP_LOGIN` и `BOOTSTRAP_PASSWORD`; после создания пользователя удалите bootstrap password из конфигурации.

## Упаковка и обслуживание

`deploy/compose.production.yml` использует заранее собранные `API_IMAGE` и `WEB_IMAGE`, Caddy на публичных 80/443 и внутренние API/PostgreSQL. Для локальной HTTP-проверки добавляется `deploy/compose.local.yml`; она публикует только loopback-порт. Production jobs имеют profiles `jobs` (migrate/seed) и `bootstrap`.

## CI и dry-run

CI включает backend race/integration tests, frontend quality/a11y, sqlc diff, каталог, image scans и локальный production smoke. Workflow `cd-dry-run.yml` проверяет SHA image references и моделирует migration/health/smoke/rollback gates; он не публикует images и не подключается по SSH. Откат БД не выполняется.

Резервные копии исключены из текущего этапа. Реальный deploy не настроен. Для него отдельно выбираются Timeweb IP/user, domain/DNS, install path, GitHub repository; SSH должен использовать выделенного непривилегированного пользователя, отдельный key и проверенные known_hosts. Перед реальным запуском нужны отдельное разрешение и проверка внешних параметров.
