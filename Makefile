.NOTPARALLEL:

GO ?= GOTOOLCHAIN=go1.27.2 go
NODE_RUN ?= npx --yes --package=node@22.22.0 -c
SEED_FILES ?= seeds/*.v1.json

.PHONY: dev stop compose-config test test-backend test-integration test-frontend test-browser seed check-seed check-seed-links lint build build-backend build-frontend

dev:
	docker compose --env-file .env -f deploy/compose.development.yml up -d

stop:
	docker compose --env-file .env -f deploy/compose.development.yml down

compose-config:
	POSTGRES_PASSWORD=validation-only docker compose -f deploy/compose.development.yml config --quiet

test: test-backend test-frontend

test-backend:
	cd backend && $(GO) test ./...

test-integration:
	cd backend && $(GO) test -tags=integration ./...

test-frontend:
	cd frontend && $(NODE_RUN) 'npm ci --ignore-scripts && npm test'

test-browser:
	cd frontend && $(NODE_RUN) 'npm ci --ignore-scripts && npm run test:browser'

check-seed:
	cd backend && $(GO) run ./cmd/check-seed $(SEED_FILES)

check-seed-links:
	cd backend && $(GO) run ./cmd/check-seed -links $(SEED_FILES)

seed:
	cd backend && $(GO) run ./cmd/seed $(SEED_FILES)

lint:
	test -z "$$(gofmt -l backend)"
	cd backend && $(GO) vet ./...
	cd frontend && $(NODE_RUN) 'npm ci --ignore-scripts && npm run lint && npm run typecheck'

build: build-backend build-frontend

build-backend:
	cd backend && $(GO) build -o /tmp/track-app-api ./cmd/api

build-frontend:
	cd frontend && $(NODE_RUN) 'npm ci --ignore-scripts && npm run build'

.PHONY: migrate migrate-up docker-build seed-catalog test-operations e2e smoke cd-dry-run

migrate:
	cd backend && MIGRATIONS_DIR=migrations $(GO) run ./cmd/migrate

migrate-up: migrate

seed-catalog:
	cd backend && $(GO) run ./cmd/check-seed -catalog $(SEED_FILES)

docker-build:
	docker build -f backend/Dockerfile -t track-app-api:local .
	docker build -f frontend/Dockerfile -t track-app-web:local .

test-operations:
	python3 -m unittest discover -s deploy -p 'test_*.py'

e2e:
	cd frontend && $(NODE_RUN) 'npm ci --ignore-scripts && if [ -z "$${PLAYWRIGHT_CHANNEL:-}" ]; then npx playwright install chromium; fi && npm run test:e2e'

smoke: docker-build
	cd frontend && $(NODE_RUN) 'npm ci --ignore-scripts && if [ -z "$${PLAYWRIGHT_CHANNEL:-}" ]; then npx playwright install chromium; fi'
	$(NODE_RUN) 'python3 deploy/operations.py local-smoke --e2e'

cd-dry-run:
	python3 deploy/operations.py release-dry-run

.PHONY: tools generate generate-check staticcheck security-scan workflow-check ci

tools:
	mkdir -p .tools
	cd backend && GOBIN=$(CURDIR)/.tools $(GO) install honnef.co/go/tools/cmd/staticcheck@v0.8.2
	cd backend && GOBIN=$(CURDIR)/.tools $(GO) install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
	cd backend && GOBIN=$(CURDIR)/.tools $(GO) install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
	cd backend && GOBIN=$(CURDIR)/.tools $(GO) install golang.org/x/vuln/cmd/govulncheck@v1.8.0

generate:
	cd backend && ../.tools/sqlc generate

generate-check:
	deploy/check-generated.sh

staticcheck:
	cd backend && ../.tools/staticcheck ./...

security-scan:
	cd backend && ../.tools/govulncheck ./...
	cd frontend && $(NODE_RUN) 'npm audit --audit-level=high'
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:0.75.0 image --scanners vuln --severity HIGH,CRITICAL --exit-code 1 track-app-api:local
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:0.75.0 image --scanners vuln --severity HIGH,CRITICAL --exit-code 1 track-app-web:local

workflow-check:
	.tools/actionlint

ci: lint test test-integration test-operations seed-catalog generate-check staticcheck build docker-build workflow-check
