# syntax=docker/dockerfile:1
FROM caddy:2-alpine

LABEL org.opencontainers.image.title=Mercure.rocks
LABEL org.opencontainers.image.description="Real-time made easy"
LABEL org.opencontainers.image.url=https://mercure.rocks
LABEL org.opencontainers.image.source=https://github.com/dunglas/mercure
LABEL org.opencontainers.image.licenses=AGPL-3.0-or-later
LABEL org.opencontainers.image.vendor="Kévin Dunglas"

ENV MERCURE_TRANSPORT_URL=bolt:///data/mercure.db

COPY mercure /usr/bin/caddy
COPY Caddyfile /etc/caddy/Caddyfile
COPY dev.Caddyfile /etc/caddy/dev.Caddyfile
