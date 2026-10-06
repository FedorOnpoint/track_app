GO_IMAGE ?= golang:1.27.1
NODE_IMAGE ?= node:22-alpine

.PHONY: test test-backend test-frontend lint build build-backend build-frontend

test: test-backend test-frontend

test-backend:
	docker run --rm -v "$(CURDIR)/backend:/app" -w /app $(GO_IMAGE) go test ./...

test-frontend:
	docker run --rm -v "$(CURDIR)/frontend:/app" -w /app $(NODE_IMAGE) sh -c "npm ci --cache /tmp/npm-cache && npm test"

lint:
	docker run --rm -v "$(CURDIR)/backend:/app" -w /app $(GO_IMAGE) sh -c "test -z \"$$(gofmt -l .)\" && go vet ./..."
	docker run --rm -v "$(CURDIR)/frontend:/app" -w /app $(NODE_IMAGE) sh -c "npm ci --cache /tmp/npm-cache && npm run lint && npm run typecheck"

build: build-backend build-frontend

build-backend:
	docker run --rm -v "$(CURDIR)/backend:/app" -w /app $(GO_IMAGE) go build -o /tmp/track-app-api ./cmd/api

build-frontend:
	docker run --rm -v "$(CURDIR)/frontend:/app" -w /app $(NODE_IMAGE) sh -c "npm ci --cache /tmp/npm-cache && npm run build"
