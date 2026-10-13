FROM golang:1.27.2 AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/bootstrap ./cmd/bootstrap && \
    CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/seed ./cmd/seed && \
    CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/migrate ./cmd/migrate && \
    CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/healthcheck ./cmd/healthcheck

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && addgroup -g 10001 app && adduser -D -u 10001 -G app app
WORKDIR /app
COPY --from=build /out/ /app/
COPY backend/migrations/ /app/migrations/
COPY backend/seeds/*.json /app/seeds/
USER 10001:10001
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=4s --start-period=10s --retries=5 CMD ["/app/healthcheck"]
ENTRYPOINT ["/app/api"]
