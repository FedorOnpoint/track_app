FROM node:22.22.0-alpine AS build
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --ignore-scripts
COPY frontend/ ./
RUN npm run build

FROM caddy:2.11.7-alpine
RUN apk add --no-cache curl && setcap -r /usr/bin/caddy && addgroup -g 10001 app && adduser -D -u 10001 -G app app && chown -R app:app /data /config
COPY --from=build /src/dist /srv
COPY deploy/Caddyfile /etc/caddy/Caddyfile
USER 10001:10001
EXPOSE 8080 8443
