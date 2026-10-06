GO ?= GOTOOLCHAIN=go1.27.1 go
NODE_RUN ?= npx --yes --package=node@22.22.0 -c

.PHONY: dev stop compose-config test test-backend test-integration test-frontend lint build build-backend build-frontend

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

lint:
	test -z "$$(gofmt -l backend)"
	cd backend && $(GO) vet ./...
	cd frontend && $(NODE_RUN) 'npm ci --ignore-scripts && npm run lint && npm run typecheck'

build: build-backend build-frontend

build-backend:
	cd backend && $(GO) build -o /tmp/track-app-api ./cmd/api

build-frontend:
	cd frontend && $(NODE_RUN) 'npm ci --ignore-scripts && npm run build'
