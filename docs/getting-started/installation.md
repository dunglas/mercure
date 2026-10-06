---
title: "Install the Mercure.rocks hub"
description: "Get started with managed Mercure Cloud, or install the Mercure hub with Docker, Kubernetes, or a binary. Deploy Mercure Enterprise on your own infrastructure."
---

# Install the Mercure hub

**The fastest way to get started is [Mercure Cloud](https://mercure.rocks/pricing).** Get a managed hub with automatic HTTPS and a custom domain. We operate the infrastructure; you build the real-time features. [Choose your Cloud plan](https://mercure.rocks/pricing).

Need to run Mercure on your own servers? **[Mercure Enterprise](../production/high-availability.md)** adds clustering, Redis/Valkey, PostgreSQL, Kafka, and Pulsar transports, plus direct support. The [Managed On-Premise option](https://mercure.rocks/pricing) lets our team deploy, monitor, and update it on your infrastructure.

To run the open-source hub yourself, choose Docker, Helm, or a prebuilt binary below. All editions speak the same Mercure protocol.

The Mercure.rocks Hub is a custom build of the [Caddy web server](https://caddyserver.com/) with the Mercure module. It includes Caddy's standard TLS, HTTP/3, compression, reverse-proxy, and metrics features. Third-party Caddy modules require a custom build.

## Docker (recommended)

Generate each key once with `openssl rand -base64 32` and export it as `MERCURE_PUBLISHER_JWT_KEY` / `MERCURE_SUBSCRIBER_JWT_KEY`: anyone can forge tokens signed with the development key.

```console
docker run \
    -e MERCURE_PUBLISHER_JWT_KEY \
    -e MERCURE_SUBSCRIBER_JWT_KEY \
    -p 80:80 -p 443:443 \
    dunglas/mercure
```

HTTPS is enabled by default. Set `SERVER_NAME` to a public hostname for automatic certificates; `localhost` uses Caddy's local certificate authority. Behind a TLS-terminating proxy, follow the [reverse-proxy configuration](../deployment/reverse-proxy.md).

For local development, set `MERCURE_EXTRA_DIRECTIVES=playground`, which enables anonymous subscriptions and the debug UI:

```console
docker run \
    -e MERCURE_EXTRA_DIRECTIVES=playground \
    -p 80:80 -p 443:443 \
    dunglas/mercure
```

The hub is then available at `https://localhost`, with the debug UI at `https://localhost/.well-known/mercure/debug/`.

The image's `HEALTHCHECK` queries the [transport-aware](../production/health-monitoring.md) `/mercure/health/ready` endpoint on the Caddy admin API.

## Docker Compose

Save this configuration as `compose.yaml`, then run `docker compose up -d`:

```yaml
services:
  mercure:
    image: dunglas/mercure
    restart: unless-stopped
    environment:
      # Uncomment to disable HTTPS (use behind a reverse proxy)
      #SERVER_NAME: ':80'
      MERCURE_PUBLISHER_JWT_KEY: ${MERCURE_PUBLISHER_JWT_KEY}
      MERCURE_SUBSCRIBER_JWT_KEY: ${MERCURE_SUBSCRIBER_JWT_KEY}
      # Uncomment to run in development mode (insecure playground)
      #MERCURE_EXTRA_DIRECTIVES: playground
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
helm repo add mercure https://charts.mercure.rocks
helm install my-release mercure/mercure
```

The chart includes settings for draining SSE connections during rolling updates. Persistent BoltDB deployments need the `Recreate` strategy; use a shared transport for rolling upgrades. See [Kubernetes deployment](../deployment/kubernetes.md) for values, probes, and rootless setup.

## Mercure hub prebuilt binary

Download an archive for your OS from the [release page](https://github.com/dunglas/mercure/releases) and extract it.

```console
MERCURE_EXTRA_DIRECTIVES='playground' \
./mercure run --config Caddyfile
```

The hub listens at `https://localhost`. For production, remove `playground`, configure publisher and subscriber verification keys, and set `SERVER_NAME` and `MERCURE_TRUSTED_ISSUERS` for your deployment.

**macOS users:** downloaded binaries may carry a quarantine attribute. If macOS blocks the downloaded binary, remove the quarantine attribute with `xattr -d com.apple.quarantine ./mercure`.

**Windows users:** allow inbound connections through Windows Defender Firewall only on the networks where you intend to expose the hub.

If port 80 or 443 is taken (Apache, NGINX, Skype), set `SERVER_NAME=:3000` (or any free port) before starting.

## Mercure on Arch Linux

```console
yay -S mercure
```

Available [on the AUR](https://aur.archlinux.org/packages/mercure). Or `makepkg -sri` against the PKGBUILD if you don't use an AUR wrapper.

## Custom Caddy build

If you need other Caddy modules in the same binary (rate limiting, OAuth, custom storage), build with [`xcaddy`](https://github.com/caddyserver/xcaddy):

```console
xcaddy build \
  --with github.com/dunglas/mercure/caddy
```

Or use the [Caddy download page](https://caddyserver.com/download?package=github.com%2Fdunglas%2Fmercure%2Fcaddy) to assemble a build in the browser.

## Embedding the Mercure hub in a Go binary

Mercure is also a Go library. See [pkg.go.dev/github.com/dunglas/mercure](https://pkg.go.dev/github.com/dunglas/mercure). You'd typically reach for it when you want to ship a hub as part of a larger Go binary; for everything else the standalone server is simpler.

A hub built without a publisher key leaves the publish endpoint unauthenticated (the protocol's closed-network deployment mode): such a hub must never be reachable from untrusted networks. The Caddy module refuses this configuration unless the embedding application opts in with `AllowNoPublish`.

## Verify the Mercure hub installation

```console
curl --fail-with-body -i http://localhost:2019/mercure/health/ready
```

Expect `200 OK` with a JSON body containing `"status":"ok"`. Run this inside the container for Docker deployments: the admin API listens on the container's loopback interface.

## Next steps

- [Quickstart](quickstart.md): first subscribe, first publish.
- [Configuration](../deployment/configuration.md): directives and environment variables.
- [Authorization](../concepts/authorization.md): issuing publisher and subscriber tokens.
