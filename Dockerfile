# syntax=docker/dockerfile:1
FROM caddy:2-alpine

LABEL org.opencontainers.image.title=Mercure.rocks
LABEL org.opencontainers.image.description="Real-time made easy"
LABEL org.opencontainers.image.url=https://mercure.rocks
LABEL org.opencontainers.image.documentation=https://mercure.rocks/docs/hub/install
LABEL org.opencontainers.image.source=https://github.com/dunglas/mercure
LABEL org.opencontainers.image.licenses=AGPL-3.0-or-later
LABEL org.opencontainers.image.vendor="Kévin Dunglas"

COPY mercure /usr/bin/caddy
COPY Caddyfile /etc/caddy/Caddyfile
COPY dev.Caddyfile /etc/caddy/dev.Caddyfile

# Re-applies the same capability the upstream caddy:2-alpine binary ships with, so the binary can bind to 80/443 when running as a non-root user.
RUN setcap cap_net_bind_service=+ep /usr/bin/caddy

# Transport-aware readiness check on the Caddy admin API (localhost:2019 by default).
HEALTHCHECK --interval=30s --timeout=5s --start-period=60s --retries=5 \
	CMD ["wget", "-q", "--spider", "http://127.0.0.1:2019/mercure/health/ready"]
