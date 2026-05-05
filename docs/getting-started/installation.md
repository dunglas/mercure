---
title: "Install the Mercure.rocks Hub"
description: "Install the Mercure.rocks Hub on Docker, Docker Compose, Kubernetes (Helm), Linux, macOS, Windows, or Arch Linux, plus custom Caddy builds."
---

# Installation

Pick the install method that matches how you ship the rest of your stack. They all run the same hub.

> **Skip the infrastructure?** [Mercure Cloud](https://mercure.rocks/pricing) is the managed version: a hub provisioned in seconds, with TLS, custom domains, and SRE on call. The free tier is sized for prototyping; paid tiers start at €35/month. Same protocol as the open-source hub, so your code doesn't change if you migrate later.

The Mercure.rocks Hub is a custom build of the [Caddy web server](https://caddyserver.com/) with the Mercure module. Anything Caddy can do, this binary can do too — TLS, HTTP/3, compression, reverse proxying, Prometheus metrics.

## Docker (Recommended)

```console
# Docker (recommended)
docker run \
    -e MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    -e MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    -p 80:80 -p 443:443 \
    dunglas/mercure
```

HTTPS is on by default — Caddy issues a Let's Encrypt certificate for the configured `SERVER_NAME`. To disable HTTPS (typically when running behind a reverse proxy), set `SERVER_NAME=:80`.

For local development, swap the entrypoint to load `dev.Caddyfile`, which enables anonymous subscriptions and the debug UI:

```console
# Docker (recommended)
docker run \
    -e MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    -e MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
    -p 80:80 -p 443:443 \
    dunglas/mercure caddy run --config /etc/caddy/dev.Caddyfile
```

The hub is then available at `https://localhost`, with the debug UI at `https://localhost/.well-known/mercure/ui/`.

The image's `HEALTHCHECK` queries the [transport-aware](../production/health-monitoring.md) `/mercure/health/ready` endpoint on the Caddy admin API.

## Docker Compose

```yaml
# compose.yaml
services:
  mercure:
    image: dunglas/mercure
    restart: unless-stopped
    environment:
      # Uncomment to disable HTTPS (use behind a reverse proxy)
      #SERVER_NAME: ':80'
      MERCURE_PUBLISHER_JWT_KEY: "!ChangeThisMercureHubJWTSecretKey!"
      MERCURE_SUBSCRIBER_JWT_KEY: "!ChangeThisMercureHubJWTSecretKey!"
    # Uncomment to run in development mode
    #command: /usr/bin/caddy run --config /etc/caddy/dev.Caddyfile
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

The `/data` volume holds the BoltDB history; `/config` holds Caddy's autosaved configuration. See [Docker deployment](../deployment/docker.md) for healthchecks and rootless mode.

## Kubernetes (Helm)

```console
# Kubernetes (Helm)
helm repo add mercure https://charts.mercure.rocks
helm install my-release mercure/mercure
```

The chart ships SSE-appropriate defaults (`terminationGracePeriodSeconds: 660`, surge updates) so rolling deploys don't reconnect every client at once. See [Kubernetes deployment](../deployment/kubernetes.md) for values, probes, and rootless setup.

## Mercure Hub Prebuilt Binary

Download an archive for your OS from the [release page](https://github.com/dunglas/mercure/releases) and extract it.

```console
# Mercure Hub Prebuilt Binary
MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
./mercure run --config dev.Caddyfile
```

The hub binds to `https://localhost`. To run in production mode (no anonymous subscribers, no debug UI), drop the `--config dev.Caddyfile` flag.

**macOS users:** the binary is quarantined on first run. Strip the attribute once with `xattr -d com.apple.quarantine ./mercure`.

**Windows users:** Windows Defender Firewall will prompt on first start. Allow on both public and private networks. Whitelist `mercure.exe` if you run additional security software.

If port 80 or 443 is taken (Apache, NGINX, Skype), set `SERVER_NAME=:3000` (or any free port) before starting.

## Mercure on Arch Linux

```console
# Mercure on Arch Linux
yay -S mercure
```

Available [on the AUR](https://aur.archlinux.org/packages/mercure). Or `makepkg -sri` against the PKGBUILD if you don't use an AUR wrapper.

## Custom Caddy Build

If you need other Caddy modules in the same binary (rate limiting, OAuth, custom storage), build with [`xcaddy`](https://github.com/caddyserver/xcaddy):

```console
# Custom Caddy build
xcaddy build \
  --with github.com/dunglas/mercure/caddy
```

Or use the [Caddy download page](https://caddyserver.com/download?package=github.com%2Fdunglas%2Fmercure%2Fcaddy) to assemble a build in the browser.

## Embedding the Mercure Hub in a Go Binary

Mercure is also a Go library. See [pkg.go.dev/github.com/dunglas/mercure](https://pkg.go.dev/github.com/dunglas/mercure). You'd typically reach for it when you want to ship a hub as part of a larger Go binary; for everything else the standalone server is simpler.

## Verify the Mercure Hub Installation

```console
# Verify the Mercure Hub Installation
curl -i https://localhost/.well-known/mercure
```

You should see `405 Method Not Allowed` — the hub only accepts `GET` (subscribe) and `POST` (publish) on this endpoint. Anything else means the hub answered.

## Mercure Installation Next Steps

- [Quickstart](quickstart.md) — first subscribe, first publish.
- [Configuration](../deployment/configuration.md) — directives and environment variables.
- [Authorization](../concepts/authorization.md) — minting JWTs that actually pass validation.
