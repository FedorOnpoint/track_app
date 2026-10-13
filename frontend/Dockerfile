FROM node:22.22.0-alpine AS build
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --ignore-scripts
COPY frontend/ ./
RUN npm run build

# Rebuild the pinned Caddy release with patched Go and HTTP/2 dependencies.
FROM golang:1.27.2 AS caddy-build
WORKDIR /src
RUN go mod init tracker-caddy && \
    go get github.com/caddyserver/caddy/v2/cmd/caddy@v2.11.7 golang.org/x/net@v0.60.0 && \
    CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/caddy github.com/caddyserver/caddy/v2/cmd/caddy

FROM caddy:2.11.7-alpine
RUN apk add --no-cache curl && setcap -r /usr/bin/caddy && addgroup -g 10001 app && adduser -D -u 10001 -G app app && chown -R app:app /data /config
COPY --from=caddy-build /out/caddy /usr/bin/caddy
COPY --from=build /src/dist /srv
COPY deploy/Caddyfile /etc/caddy/Caddyfile
USER 10001:10001
EXPOSE 8080 8443
