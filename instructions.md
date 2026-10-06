# План создания Go Middle Interview Tracker

## 1. Роль агента и ожидаемый результат

Ты — senior full-stack инженер и технический редактор материалов по Go. Создай production-ready личное веб-приложение для подготовки одного пользователя к собеседованию на позицию Middle Go Backend Developer.

Приложение не должно содержать встроенную среду исполнения кода, автоматическую проверку задач, социальные функции, регистрацию новых пользователей, роли, платежи или админ-панель. Его назначение — структурировать теорию, вопросы для самопроверки, внешние материалы, рекомендуемую практику, заметки и прогресс обучения.

Работай итеративно и сначала изучи этот документ целиком. Не выполняй Git commit, push, создание pull request и реальный деплой без отдельного явного разрешения пользователя. Не помещай секреты в репозиторий, логи, Docker-образы или frontend bundle.

## 2. Цели MVP

Пользователь должен иметь возможность:

1. Войти по логину и паролю.
2. Просматривать готовую карту знаний Middle Go Backend Developer.
3. Открывать темы с краткой и расширенной теорией.
4. Читать типовые вопросы, ожидаемые ответы, follow-up вопросы и распространённые ошибки.
5. Видеть рекомендации, что прочитать, посмотреть и где выполнить внешние упражнения.
6. Искать и фильтровать учебные материалы.
7. Присваивать теме статус и видеть общий прогресс.
8. Помечать темы для повторения и добавлять закладки.
9. Создавать и редактировать личные заметки, связанные с темами.
10. Безопасно обновлять приложение на сервере Timeweb через CI/CD.

## 3. Зафиксированный стек

Перед началом реализации проверь официальные release notes и используй текущие стабильные, поддерживаемые версии. Не используй beta, RC и experimental API без необходимости.

Ориентир на 6 октября 2026 года:

- Backend: Go 1.27.x, стандартный `net/http`, `chi` v5 для маршрутизации.
- Database: PostgreSQL 18.x с актуальным minor-релизом.
- Драйвер БД: `pgx/v5`; типобезопасные запросы генерировать через `sqlc`.
- Миграции: `golang-migrate` или эквивалентный зрелый инструмент с поддержкой up/down migrations.
- Frontend: React 19.x, TypeScript, Vite 8.x.
- Получение серверного состояния: TanStack Query; маршрутизация: React Router.
- Формы и валидация: React Hook Form + Zod.
- Стили: Tailwind CSS; интерфейс должен быть доступным и адаптивным.
- Тесты backend: стандартный `testing`, `httptest`, Testcontainers for Go для PostgreSQL integration tests.
- Тесты frontend: Vitest + React Testing Library; Playwright — только для критического smoke flow.
- Production: Docker Compose, Caddy 2, GitHub Container Registry, GitHub Actions.

Если совместимость актуальных major-версий отличается от указанной, выбери последнюю стабильную совместимую комбинацию, зафиксируй версии lock-файлами и кратко объясни отклонение пользователю до реализации.

## 4. Архитектура

Используй modular monolith, а не микросервисы.

```text
Browser
  -> Caddy (TLS, security headers, static frontend)
      -> /api/* -> Go API
                    -> PostgreSQL
```

- Frontend собирается в статические файлы и отдаётся Caddy.
- Backend предоставляет JSON REST API под `/api/v1`.
- PostgreSQL не публикует порт наружу и доступен только во внутренней Docker-сети.
- Caddy — единственный контейнер с публичными портами 80 и 443.
- Backend остаётся stateless; пользовательские сессии хранятся в PostgreSQL.
- Контент, прогресс и заметки хранятся в PostgreSQL.
- Начальный учебный контент загружается идемпотентной seed-командой.

Не вводи Redis, message broker, Kubernetes, GraphQL, SSR и отдельные сервисы: для одного пользователя это неоправданное усложнение.

## 5. Предлагаемая структура репозитория

```text
.
├── backend/
│   ├── cmd/api/main.go
│   ├── cmd/seed/main.go
│   ├── internal/auth/
│   ├── internal/catalog/
│   ├── internal/progress/
│   ├── internal/notes/
│   ├── internal/bookmarks/
│   ├── internal/platform/config/
│   ├── internal/platform/database/
│   ├── internal/platform/httpserver/
│   ├── migrations/
│   ├── queries/
│   ├── seeds/
│   ├── sqlc.yaml
│   ├── go.mod
│   └── Dockerfile
├── frontend/
│   ├── src/app/
│   ├── src/pages/
│   ├── src/features/
│   ├── src/entities/
│   ├── src/shared/
│   ├── package.json
│   └── Dockerfile
├── deploy/
│   ├── Caddyfile
│   ├── compose.production.yml
│   ├── compose.development.yml
│   └── backup-postgres.sh
├── .github/workflows/ci.yml
├── .github/workflows/deploy.yml
├── .env.example
├── Makefile
└── README.md
```

Файл может быть разделён дополнительно, если становится трудно обозримым. Не создавай абстракции, у которых нет минимум двух реальных потребителей.

## 6. Учебная программа и seed-контент

Подготовь содержательную базу уровня Middle, а не только названия разделов. Язык интерфейса и основных материалов — русский; общепринятые технические термины оставляй на английском, если перевод ухудшает точность.

Обязательные категории:

1. Go Core: типы, zero values, массивы, slices, maps, строки, функции, методы, interfaces, embedding, errors, panic/recover, generics, packages, modules.
2. Runtime и память: stack/heap, escape analysis, allocation, GC, scheduler, GMP, `GOMAXPROCS`, data races, profiling.
3. Конкурентность: goroutines, channels, `select`, cancellation, context, mutex/RWMutex, atomic, WaitGroup, worker pool, pipelines, backpressure, leak prevention.
4. Standard Library: `net/http`, `io`, `time`, `sync`, `encoding/json`, `database/sql`, filesystem, testing.
5. HTTP и API: методы и статусы, REST, middleware, validation, error contracts, idempotency, pagination, versioning, timeouts, retries, rate limiting.
6. PostgreSQL и SQL: моделирование, joins, CTE, window functions, indexes, `EXPLAIN ANALYZE`, transactions, isolation, locks, deadlocks, MVCC, connection pools, migrations.
7. Архитектура: cohesion/coupling, dependency inversion, DI, repository pattern, modular monolith, clean/hexagonal architecture без догматизма, boundaries и trade-offs.
8. Тестирование: table-driven tests, subtests, mocks/fakes, integration tests, testcontainers, race detector, benchmarks, fuzzing, test pyramid.
9. Инфраструктура: Linux basics, processes/signals, networking, DNS, Docker, Git, configuration, CI/CD.
10. Production: structured logging, metrics, tracing, health/readiness, graceful shutdown, pprof, SLI/SLO, incident basics.
11. Безопасность: authentication/authorization, password hashing, cookies, CSRF, CORS, TLS, SQL injection, SSRF, secrets, OWASP basics.
12. System Design: caching, queues, load balancing, replication, partitioning, consistency, availability, scaling, failure modes.
13. Поведение на собеседовании: объяснение решений, уточнение требований, оценка trade-offs, разбор production incident и code review вопросов.

Целевой объём seed-контента MVP: 80–120 тем суммарно. Для каждой темы обязательно подготовь:

- уникальный стабильный `slug`;
- название, категорию, порядок и уровень сложности;
- ожидаемые prerequisites;
- краткое резюме;
- структурированную расширенную теорию;
- 3–7 ключевых тезисов для checklist;
- 3–8 вопросов для собеседования;
- краткий и развёрнутый ожидаемый ответ на каждый вопрос;
- минимум один follow-up там, где он естественен;
- распространённые ошибки или заблуждения;
- 1–4 качественных внешних ресурса;
- необязательные внешние упражнения с пояснением результата, который нужно получить.

Не выдумывай URL. Предпочитай официальную документацию Go, PostgreSQL, Docker, MDN, OWASP и первичные источники. Проверь, что каждая ссылка доступна на момент наполнения. Не копируй большие фрагменты чужих материалов: пиши оригинальные краткие объяснения и указывай источник.

Seed должен быть версионирован. Повторный запуск обновляет системный контент по стабильным ключам, не создаёт дубликаты и не удаляет пользовательские статусы, заметки и закладки.

## 7. Экраны и UX

### Вход

- Поля login/password, понятные ошибки без раскрытия существования пользователя.
- После успешного входа — редирект на главную.
- Неавторизованные запросы к приватным маршрутам возвращают 401; UI переводит на экран входа.

### Главная

- Общий процент изучения.
- Прогресс по категориям.
- Темы со статусом «нужно повторить».
- Недавно изменённые темы и заметки.
- Кнопка продолжения последней открытой темы.

### Карта знаний

- Категории и темы с прогрессом.
- Поиск по названию, summary, теории, вопросам и ключевым словам.
- Фильтры по категории, сложности, статусу, закладке и необходимости повторения.
- Пустые состояния и возможность сбросить фильтры.

### Страница темы

- Навигация по разделам.
- Краткая и расширенная теория.
- Checklist, вопросы/ответы, ошибки, материалы и внешняя практика.
- Управление статусом, закладкой и отметкой повторения.
- Личные заметки с созданием, редактированием и удалением с подтверждением.

### Трек-лист

- Компактная таблица всех тем.
- Статусы: `not_started`, `learning`, `review`, `mastered`.
- Массовое изменение статусов не входит в MVP.

### Закладки и повторение

- Отдельные списки с поиском и фильтрами.
- Из списка можно сразу открыть тему и изменить статус.

### Настройки

- Смена пароля после проверки текущего пароля.
- Logout со всех текущих сессий.

Интерфейс должен корректно работать на desktop и mobile, иметь keyboard navigation, видимый focus, semantic HTML, достаточный contrast и поддержку `prefers-reduced-motion`. Не трать время на сложные анимации.

## 8. Модель данных

Используй UUID или PostgreSQL identity для внутренних ID последовательно. Для публичных URL тем используй `slug`.

Минимальные таблицы:

- `users`: id, login unique, password_hash, created_at, updated_at.
- `sessions`: id, user_id, token_hash unique, expires_at, created_at, last_seen_at.
- `categories`: id, slug unique, title, description, sort_order.
- `topics`: id, category_id, slug unique, title, summary, difficulty, sort_order, estimated_minutes, content_version, created_at, updated_at.
- `topic_sections`: id, topic_id, heading, body_markdown, sort_order.
- `checklist_items`: id, topic_id, stable_key unique within topic, text, sort_order.
- `interview_questions`: id, topic_id, stable_key, question, short_answer, detailed_answer, pitfalls, sort_order.
- `follow_up_questions`: id, interview_question_id, question, expected_answer, sort_order.
- `resources`: id, topic_id, title, url, resource_type, source_name, sort_order.
- `practice_recommendations`: id, topic_id, title, description, url nullable, expected_outcome, sort_order.
- `topic_progress`: user_id, topic_id, status, review_required, last_opened_at, updated_at; unique(user_id, topic_id).
- `checklist_progress`: user_id, checklist_item_id, completed, updated_at; unique(user_id, checklist_item_id).
- `bookmarks`: user_id, topic_id, created_at; unique(user_id, topic_id).
- `notes`: id, user_id, topic_id, body_markdown, created_at, updated_at.

Ограничь значения `difficulty`, `resource_type` и `status` через PostgreSQL constraints либо lookup types. Добавь внешние ключи и индексы по реальным фильтрам. Удаление системной темы через пользовательский API не предоставляй.

## 9. API-контракт

Базовый путь: `/api/v1`. JSON использует `snake_case`. Время передаётся в RFC 3339 UTC.

Обязательные endpoints:

```text
POST   /auth/login
POST   /auth/logout
POST   /auth/logout-all
GET    /auth/me
PUT    /auth/password

GET    /dashboard
GET    /categories
GET    /topics
GET    /topics/{slug}
PUT    /topics/{slug}/progress
PUT    /topics/{slug}/checklist/{item_id}
PUT    /topics/{slug}/bookmark
DELETE /topics/{slug}/bookmark

GET    /bookmarks
GET    /review
GET    /notes
POST   /topics/{slug}/notes
PUT    /notes/{id}
DELETE /notes/{id}

GET    /health/live
GET    /health/ready
```

`GET /topics` поддерживает `query`, `category`, `difficulty`, `status`, `bookmarked`, `review_required`, `page`, `page_size`, `sort`. Ограничь `page_size`, используй детерминированную сортировку и валидируй все параметры.

Единый формат ошибки:

```json
{
  "error": {
    "code": "validation_error",
    "message": "Проверьте введённые данные",
    "fields": {"password": "Пароль слишком короткий"},
    "request_id": "..."
  }
}
```

Не отправляй stack traces, SQL, секреты или внутренние детали клиенту.

## 10. Аутентификация и безопасность

- Одна учётная запись создаётся отдельной bootstrap-командой или при первом деплое из одноразовых переменных окружения.
- Хешируй пароль Argon2id с параметрами, выбранными по актуальной рекомендации OWASP; соль генерируется криптографически безопасно.
- Сессионный токен — минимум 256 бит энтропии. В БД хранится только SHA-256/другой безопасный односторонний hash токена.
- Cookie: `HttpOnly`, `Secure` в production, `SameSite=Lax`, `Path=/`, ограниченный срок жизни.
- Ротируй сессию после входа и смены пароля. После смены пароля инвалидируй остальные сессии.
- Добавь rate limiting на login и искусственно одинаковый ответ для неверного login/password.
- Для state-changing запросов проверяй Origin/Referer и применяй CSRF-защиту, совместимую с выбранной cookie-схемой.
- CORS не нужен при одном origin; не разрешай `*` вместе с credentials.
- Установи CSP, HSTS после проверки HTTPS, `X-Content-Type-Options`, `Referrer-Policy` и ограничение размера request body.
- Markdown выводи через безопасный renderer с sanitization; raw HTML отключи.
- Логи не должны содержать пароль, cookie, session token или полный текст заметок.

## 11. Конфигурация

Приложение должно стартовать с проверкой конфигурации и понятной ошибкой при отсутствии обязательных значений.

Пример переменных без реальных секретов:

```text
APP_ENV=production
APP_ADDR=:8080
APP_DOMAIN=interview.example.com
DATABASE_URL=postgres://...
BOOTSTRAP_LOGIN=...
BOOTSTRAP_PASSWORD=...
SESSION_TTL=168h
LOG_LEVEL=info
```

`BOOTSTRAP_PASSWORD` используется одноразово и после создания/смены пароля удаляется из production `.env`. Подготовь `.env.example`, но никогда не добавляй настоящий `.env` в Git.

## 12. Наблюдаемость и надёжность

- Structured JSON logs с request ID, method, path template, status, duration; не логируй query с потенциально личными данными.
- Graceful shutdown по SIGTERM/SIGINT.
- Настрой server read/write/idle timeouts и database pool limits.
- Liveness проверяет процесс; readiness проверяет возможность выполнить короткий запрос к БД.
- Ошибки записываются с причиной на backend, клиент получает безопасное сообщение.
- Добавь защиту от двойной отправки и optimistic UI только там, где откат очевиден.

## 13. Docker и Timeweb

Development Compose должен поднимать PostgreSQL и приложения с удобным local workflow. Production Compose должен использовать заранее собранные immutable images, restart policy, healthchecks, внутреннюю сеть и named volumes.

Production-сервисы:

- `caddy`;
- `api`;
- `postgres`;
- одноразовый `migrate` profile/service;
- одноразовый `seed` profile/service.

Не запускай backend и PostgreSQL от root, если официальный image позволяет задать непривилегированного пользователя. Добавь resource-conscious настройки, пригодные для небольшого Timeweb VPS. Не публикуй порты API и PostgreSQL на host interface.

Перед реальным деплоем запроси у пользователя:

- IP и SSH user сервера Timeweb;
- доменное имя и подтверждение DNS A/AAAA records;
- путь установки на сервере;
- имя GitHub repository/organization;
- способ хранения резервных копий;
- разрешение на изменение firewall и установку Docker.

Ничего из этого не угадывай.

## 14. CI/CD

### CI для pull request и main

Запускай параллельно, где возможно:

1. Backend: `gofmt` check, `go vet`, `staticcheck`, unit tests, integration tests с PostgreSQL, `go test -race ./...`.
2. Frontend: clean install по lock-файлу, typecheck, lint, unit/component tests, production build.
3. Проверка генерации: `sqlc generate` не должен оставлять diff.
4. Docker build обоих production images.
5. Dependency/security scanning без автоматического раскрытия секретов.

### CD после успешного main

1. Собрать images и отправить в GHCR с immutable tag по commit SHA; не полагаться только на `latest`.
2. Подключиться к Timeweb по SSH отдельным deploy key/user.
3. Скачать новый compose/image reference.
4. Выполнить backup БД.
5. Запустить миграции; при ошибке остановить деплой.
6. Выполнить `docker compose pull` и `docker compose up -d`.
7. Дождаться healthchecks и выполнить внешний HTTPS smoke test.
8. При неуспехе приложения вернуть предыдущие image tags. Не пытаться автоматически откатывать необратимую миграцию данных.
9. Удалять только старые application images по безопасной retention policy; не удалять volumes.

GitHub Secrets должны содержать только необходимые значения. Ограничь SSH key и права deploy user. Не устанавливай self-hosted GitHub runner на production VPS.

## 15. Резервное копирование

- Ежедневный `pg_dump` в custom format с timestamp.
- Retention: 7 ежедневных и 4 еженедельных копии как начальный вариант.
- Копия не считается резервной, если хранится только на том же диске: предусмотри выгрузку в выбранное пользователем внешнее S3-compatible storage.
- Шифруй передачу и storage; не записывай пароль БД в командную строку или лог.
- Подготовь и проверь процедуру восстановления в отдельную test database.
- Автоматический deploy не должен удалять backups или database volume.

## 16. Тестовая стратегия

Следуй TDD для доменной логики и API. Каждый этап должен завершаться работающей проверяемой системой.

Обязательные backend-сценарии:

- корректный и ошибочный login, rate limit, истечение и отзыв сессии;
- невозможно получить приватные данные без сессии;
- пользователь может читать только свои заметки и прогресс;
- фильтры, пагинация и детерминированная сортировка;
- повторный seed не создаёт дубликаты и сохраняет пользовательские данные;
- вычисление прогресса для пустой и заполненной категории;
- конкурентные обновления статуса не повреждают данные;
- invalid enum/slug/UUID/body size дают контролируемые 4xx;
- readiness становится false при недоступной БД.

Обязательные frontend-сценарии:

- login и обработка 401;
- loading/error/empty/success states;
- поиск и фильтры синхронизированы с URL;
- изменение статуса обновляет прогресс и корректно откатывается при ошибке;
- создание/редактирование/удаление заметки;
- keyboard navigation и доступные labels для критических форм.

Минимальный E2E smoke test:

1. Войти.
2. Найти тему `goroutines`.
3. Открыть её и изменить статус.
4. Добавить заметку и закладку.
5. Убедиться, что изменения сохранились после перезагрузки.

## 17. Пошаговый план реализации

### Этап 1. Каркас и локальная инфраструктура

- Создать структуру backend/frontend/deploy.
- Настроить PostgreSQL development Compose, конфигурацию и Makefile-команды.
- Добавить базовые CI-проверки.
- Результат: чистый проект воспроизводимо собирается, тесты запускаются одной командой.

### Этап 2. Схема БД и каталог знаний

- Написать миграции, SQL-запросы и integration tests.
- Реализовать read-only категории, список тем и страницу темы.
- Создать маленький seed fixture для тестов.
- Результат: API отдаёт каталог из PostgreSQL с фильтрами и пагинацией.

### Этап 3. Аутентификация

- Реализовать bootstrap пользователя, Argon2id, login/logout/logout-all, session middleware и смену пароля.
- Добавить security tests и rate limiting.
- Результат: все приватные endpoints защищены, сессии можно отозвать.

### Этап 4. Прогресс, checklist, закладки и заметки

- Реализовать доменную логику, endpoints и integration tests.
- Добавить dashboard aggregates.
- Результат: пользовательские изменения сохраняются и правильно отражаются в статистике.

### Этап 5. Frontend shell и основные страницы

- Реализовать auth flow, layout, routing, API client и error handling.
- Создать Dashboard, Knowledge Map, Topic и Track List.
- Результат: основной путь подготовки полностью доступен из UI.

### Этап 6. Дополнительные пользовательские функции

- Добавить Review, Bookmarks, Notes и Settings.
- Реализовать адаптивность и accessibility review.
- Результат: все функции MVP доступны на desktop и mobile.

### Этап 7. Полный seed-контент

- Последовательно наполнить 13 категорий.
- Провести техническую редактуру, проверить ссылки и удалить дубли.
- Проверить идемпотентное обновление поверх существующего прогресса.
- Результат: 80–120 качественно заполненных тем, пригодных для самостоятельной подготовки.

### Этап 8. Production packaging

- Создать multi-stage Dockerfiles, production Compose и Caddyfile.
- Добавить healthchecks, migrations/seed jobs и backup/restore scripts.
- Результат: production stack проходит локальный smoke test.

### Этап 9. CI/CD для Timeweb

- Настроить CI, публикацию SHA-tagged images в GHCR и контролируемый SSH deploy.
- Добавить backup, health gate и rollback приложения.
- Результат: workflow проверяется в dry-run; реальный запуск — только после разрешения и предоставления секретов.

### Этап 10. Финальная проверка

- Запустить format/lint/typecheck/unit/integration/race/E2E/build.
- Проверить отсутствие секретов и открытых database ports.
- Проверить backup restore и документацию оператора.
- Результат: выполнены все критерии приёмки ниже.

## 18. Критерии приёмки MVP

- Все обязательные страницы и API endpoints реализованы.
- В БД находится 80–120 содержательных тем по всем 13 категориям.
- Пользователь может искать, фильтровать и изучать темы без перезагрузки состояния.
- Статусы, checklist, review flag, закладки и заметки сохраняются после restart контейнеров.
- Повторный seed не создаёт дублей и не уничтожает пользовательские данные.
- Пароль и session tokens не хранятся в открытом виде.
- Невалидные данные и инфраструктурные ошибки обрабатываются предсказуемо.
- Все автоматические проверки проходят.
- Production Compose стартует на чистом сервере после настройки переменных и DNS.
- Снаружи доступны только 80/443; HTTPS включён; PostgreSQL и API напрямую не опубликованы.
- CI не деплоит, если тесты или сборка упали.
- Перед обновлением создаётся backup, после обновления проходит health/smoke gate.
- В репозитории и истории Git нет секретов.

## 19. Порядок взаимодействия агента с пользователем

1. Перед кодом кратко перескажи понимание задачи и покажи выбранные версии/зависимости.
2. Осмотри существующий репозиторий и соблюдай локальный `AGENTS.md`, если он появится.
3. Если репозиторий пуст, предложи точную структуру без добавления лишних сервисов.
4. Реализуй этапы по порядку, используя тесты до или вместе с production code.
5. После каждого этапа сообщай: что изменено, какими командами проверено, что осталось.
6. Не заявляй об успехе без свежего вывода соответствующих тестов.
7. Не выполняй Git commit/push/deploy без отдельного прямого запроса.
8. Для реального Timeweb deployment сначала запроси недостающие параметры из раздела 13 и отдельное разрешение.
9. Если полное наполнение 80–120 тем не помещается в один безопасный шаг, выпускай его категориями, но каждая принятая категория должна быть полностью заполнена и проверена.

## 20. Команды проверки, которые должны существовать

Точные реализации выбери в Makefile/package scripts, но обеспечь единый интерфейс:

```text
make dev
make stop
make migrate-up
make seed
make test
make test-backend
make test-frontend
make lint
make build
make e2e
make smoke
make backup
make restore-check
```

Команды должны завершаться ненулевым кодом при ошибке и не зависеть от глобально установленных неописанных инструментов.

## 21. Definition of Done для каждого этапа

Этап считается завершённым только если:

- поведение покрыто тестом соответствующего уровня;
- тест сначала подтверждал отсутствие/ошибку нового поведения либо имеется объяснение, почему red phase неприменима;
- форматирование, линтеры и относящиеся к изменению тесты проходят;
- конфигурация и ошибки безопасны;
- нет временных файлов, закомментированного мёртвого кода и секретов;
- пользователь получил краткий отчёт с фактическими командами проверки;
- изменения оставлены незакоммиченными, пока пользователь явно не попросит обратное.

## 22. Работа с `addyosmani/agent-skills`

При реализации используй установленный набор skills из репозитория `addyosmani/agent-skills`. Это greenfield-проект, поэтому применяй полный жизненный цикл, но загружай skills по фазам, а не все одновременно.

Если работа выполняется в Codex, вызывай native skills через `@skill-name`. Не используй Claude slash-команды `/spec`, `/plan`, `/build`, `/review`, `/ship` как будто они доступны в Codex. Не копируй содержимое `using-agent-skills/SKILL.md` в `AGENTS.md` или постоянный prompt: Codex сам выполняет progressive disclosure и маршрутизацию skills.

### Приоритет инструкций

При конфликте соблюдай следующий порядок:

1. Прямые указания пользователя в текущем разговоре.
2. Локальный `AGENTS.md` и этот `instructions.md`.
3. Инструкции выбранного skill.
4. Общие привычки и предпочтения агента.

Следовательно, рекомендации skills делать частые commits не дают разрешения на Git commit. `/build auto` и любой workflow, автоматически создающий commits, запрещены, пока пользователь отдельно и явно не разрешит commits. То же относится к push, pull request, изменению внешней инфраструктуры и deployment.

### Фаза A. Уточнение и спецификация

Используй:

- `@using-agent-skills` — только если среда не имеет native skill routing; в Codex не загружай его вручную без необходимости;
- `@context-engineering` — собрать релевантный контекст и подготовить краткие проектные правила;
- `@spec-driven-development` — проверить этот документ как исходную спецификацию, явно перечислить допущения и открытые вопросы;
- `@source-driven-development` — подтвердить версии, API библиотек и инфраструктурные решения официальными первичными источниками.

Этот `instructions.md` является исходным product/architecture brief, но не отменяет approval gates из `spec-driven-development`. Если skill создаёт отдельный `SPEC.md`, не переписывай требования произвольно и не начинай планирование в том же ходе: покажи пользователю спецификацию и дождись одобрения. Не создавай дублирующий spec, если используемая среда допускает считать этот документ утверждённым spec artifact — сначала сообщи пользователю выбранный вариант.

### Фаза B. Планирование

После одобрения спецификации используй:

- `@planning-and-task-breakdown` — разбить работу на небольшие проверяемые vertical slices с зависимостями, acceptance criteria и командами проверки;
- `@api-and-interface-design` — зафиксировать REST-контракт, error semantics, pagination и границы модулей до реализации handlers;
- `@constraint-driven-development` — только если проекту действительно нужны отдельные измеримые quality thresholds; не создавай процесс ради процесса.

План должен учитывать этапы 1–10 этого документа. Не объединяй весь backend, весь frontend или весь seed-контент в одну задачу. Предпочитай slices, после которых можно продемонстрировать законченное поведение через API или UI.

После создания плана остановись, покажи его пользователю и дождись разрешения на реализацию. Наличие этого файла само по себе не является разрешением писать код.

### Фаза C. Реализация

Для каждого одобренного slice используй:

- `@incremental-implementation` — реализовывать один небольшой slice за раз;
- `@test-driven-development` — соблюдать Red → Green → Refactor для логики и поведения;
- `@source-driven-development` — сверять спорные framework/library решения с официальной документацией;
- `@frontend-ui-engineering` — при работе с React UI, responsive layout и accessibility;
- `@security-and-hardening` — для auth, cookies, user input, Markdown, secrets и database boundaries.

После каждого slice запускай относящиеся к нему тесты и сообщай фактический результат. Не имитируй red phase и не утверждай, что тест падал или проходил, если команда не запускалась. Если TDD неприменим к чистой конфигурации или статическому контенту, объясни это и используй подходящую проверку schema/build/smoke.

### Фаза D. Диагностика и проверка

Используй:

- `@debugging-and-error-recovery` — при любом неожиданном поведении, падении теста или build failure; сначала воспроизвести и локализовать причину, затем исправлять;
- `@browser-testing-with-devtools` — проверить реальные DOM, console, network, accessibility и ключевые flows в запущенном приложении;
- `@code-review-and-quality` — перед запросом на merge либо перед заявлением о готовности этапа;
- `@code-simplification` — только после прохождения тестов и только с сохранением поведения;
- `@doubt-driven-development` — для критичных решений в auth, migrations, backup/restore и deployment.

Review findings классифицируй по серьёзности. Не исправляй unrelated code и не скрывай дефекты отключением линтеров, тестов или security checks.

### Фаза E. Production и выпуск

Используй:

- `@ci-cd-and-automation` — при создании GitHub Actions и deployment pipeline;
- `@observability-and-instrumentation` — для logs, health/readiness и production diagnostics;
- `@shipping-and-launch` — только после явного разрешения на подготовку или выполнение реального запуска;
- `@performance-optimization` — только после измерений и при наличии проблемы или согласованной цели.

Перед deployment отдельно запроси значения и разрешения из раздела 13. Skill `shipping-and-launch` не расширяет полномочия агента и не разрешает самостоятельно менять Timeweb, DNS, firewall, GitHub Secrets или production data.

### Skills, которые не нужны по умолчанию

- `deprecation-and-migration` — только если появится реально заменяемая старая версия или breaking migration.
- `documentation-and-adrs` — только для документации, непосредственно необходимой проекту и явно разрешённой пользователем; не создавай отчёты и summary-файлы ради фиксации работы.
- `performance-optimization` — не оптимизируй без измерений.
- Specialist personas (`code-reviewer`, `test-engineer`, `security-auditor`) — подключай только если среда их поддерживает и пользователь разрешил соответствующую multi-agent работу. Skills не должны самостоятельно запускать personas.

### Проверка корректного применения skills

В отчёте по каждому этапу укажи:

1. Какие skills были применены и по какой причине.
2. Какие обязательные gates они ввели.
3. Какие команды реально запускались и каков результат.
4. Какие риски или открытые вопросы остались.
5. Что требует следующего явного разрешения пользователя.

Не считай применение skill самостоятельным результатом: ценность подтверждается созданным артефактом, тестами, runtime evidence или review findings.
