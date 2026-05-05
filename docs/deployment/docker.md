---
title: "Run the Mercure.rocks Hub with Docker and Docker Compose"
description: "Run the Mercure.rocks Hub with the official Docker image, Docker Compose, healthchecks, and rootless deployment."
---

# Docker

The official image is `dunglas/mercure`. Built on top of the [Caddy image](https://hub.docker.com/_/caddy), so anything Caddy's image supports works here too.

## Run the Mercure Docker Image

Production mode:

```console
# Run the Mercure Docker Image
docker run \
    -e MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    -e MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    -p 80:80 -p 443:443 \
    dunglas/mercure
```

The hub binds to `:80` and `:443`. Caddy issues a Let's Encrypt cert for `SERVER_NAME` automatically. Don't set `SERVER_NAME=localhost` in production: it can't be issued a public certificate.

Behind a reverse proxy that handles TLS, set `SERVER_NAME=:80` and skip `:443`:

```console
# Run the Mercure Docker Image
docker run \
    -e SERVER_NAME=':80' \
    -e MERCURE_PUBLISHER_JWT_KEY='...' \
    -e MERCURE_SUBSCRIBER_JWT_KEY='...' \
    -p 80:80 \
    dunglas/mercure
```

## Mercure Docker Development Mode

```console
# Mercure Docker Development Mode
docker run \
    -e MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    -e MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    -p 80:80 -p 443:443 \
    dunglas/mercure caddy run --config /etc/caddy/dev.Caddyfile
```

The dev Caddyfile turns on:

- the debug UI at `/.well-known/mercure/ui/`,
- anonymous subscribers,
- demo endpoints,
- a permissive CORS config (`cors_origins *`).

Don't expose this to the internet.

## Compose

```yaml
# compose.yaml
services:
  mercure:
    image: dunglas/mercure
    restart: unless-stopped
    environment:
      MERCURE_PUBLISHER_JWT_KEY: "!ChangeThisMercureHubJWTSecretKey!"
      MERCURE_SUBSCRIBER_JWT_KEY: "!ChangeThisMercureHubJWTSecretKey!"
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - mercure_data:/data
      - mercure_config:/config

volumes:
  mercure_data:
  mercure_config:
```

| Volume    | What's in it                                                         |
| --------- | -------------------------------------------------------------------- |
| `/data`   | BoltDB history (`mercure.db`) and Caddy data (autosave, cert cache). |
| `/config` | Caddy autosaved configuration.                                       |

Persist both. Losing `/data` means losing replay history; losing `/config` means re-issuing certificates on next boot.

## Mercure Docker Healthcheck

The image's built-in healthcheck queries `localhost:2019/mercure/health/ready`: the [transport-aware](../production/health-monitoring.md) readiness endpoint, not just "is the process up."

For Compose, override or extend it:

```yaml
# Mercure Docker Healthcheck
services:
  mercure:
    # ...
    healthcheck:
      test:
        [
          "CMD",
          "wget",
          "-q",
          "--spider",
          "http://localhost:2019/mercure/health/ready",
        ]
      timeout: 5s
      retries: 5
      start_period: 60s
```

The `start_period` matters: BoltDB takes a moment to open on first boot, so the first probe may fail; treat that as "not unhealthy" for the first minute.

## Rootless Mercure on Docker

The image runs as `root` by default. Recent Docker (20.10+) sets `net.ipv4.ip_unprivileged_port_start=0` inside the container, so an unprivileged process can still bind 80/443 directly.

To run as a non-root user:

```yaml
# compose.yaml
services:
  mercure:
    image: dunglas/mercure
    user: "1000:1000"
    read_only: true
    tmpfs:
      - /tmp
    environment:
      MERCURE_PUBLISHER_JWT_KEY: "!ChangeThisMercureHubJWTSecretKey!"
      MERCURE_SUBSCRIBER_JWT_KEY: "!ChangeThisMercureHubJWTSecretKey!"
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - mercure_data:/data
      - mercure_config:/config
```

The volumes must be writable by UID 1000. For fresh named volumes, set ownership once:

```console
# Rootless Mercure on Docker
docker run --rm -v mercure_data:/data -v mercure_config:/config alpine chown 1000:1000 /data /config
```

For bind mounts, `chown 1000:1000` the host directory.

## Custom Caddyfile

Ship your own `Caddyfile`:

```yaml
# compose.yaml
services:
  mercure:
    image: dunglas/mercure
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - mercure_data:/data
      - mercure_config:/config
```

A custom Caddyfile is the right move once you need:

- Multiple sites or hostnames on the same hub.
- Reverse proxying alongside the hub (Caddy handling both).
- Per-route rate limiting, request transformation, or custom auth.
- Reading secrets from files (`{file./run/secrets/jwt_key}`) instead of environment variables.

## Mercure Hub Docker Logs

Caddy logs to stdout in JSON by default. Pipe to whatever your platform expects (Loki, Datadog, CloudWatch). Useful fields:

- `mercure.subscribers_*`: connection lifecycle.
- `mercure.update_*`: publish events.
- `caddy.error_*`: TLS, listener, and transport errors.

Bump verbosity with `GLOBAL_OPTIONS=debug` (don't leave it on in prod: it logs update payloads).

## Mercure Docker Image Variants

- `dunglas/mercure`: Alpine-based, statically linked.
- `dunglas/mercure:<version>`: pin to a specific release.
- Self-Hosted ships its own image with the multi-node transports: see [High availability](../production/high-availability.md).

## Behind a Reverse Proxy

If you're already running Traefik or NGINX, terminate TLS there and let the hub speak HTTP. See [Reverse proxies](reverse-proxy.md).

## Next Steps for Mercure on Docker

- [Configuration](configuration.md): directives and env vars.
- [Kubernetes](kubernetes.md): same image, Helm chart.
- [Health monitoring](../production/health-monitoring.md): what the probes actually check.
