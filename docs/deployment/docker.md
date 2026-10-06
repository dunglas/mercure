---
title: "Run the Mercure.rocks hub with Docker and Docker Compose"
description: "Run the Mercure.rocks Hub with the official Docker image, Docker Compose, healthchecks, and rootless deployment."
---

# Run Mercure with Docker

Run `dunglas/mercure` to start the Mercure hub with Caddy. The image supports the bundled environment variables and custom Caddyfiles.

Prefer to skip server maintenance? **[Mercure Cloud](https://mercure.rocks/pricing) runs the hub for you.** For a supported cluster on your own servers, use the [Mercure Enterprise image](../production/high-availability.md#self-hosted-mercure-multi-node-on-your-infrastructure).

## Run the Mercure Docker image

Set the following values for your deployment. Generate each key once with `openssl rand -base64 32` and export it as `MERCURE_PUBLISHER_JWT_KEY` / `MERCURE_SUBSCRIBER_JWT_KEY`: anyone can forge tokens signed with the development key.

```console
docker run \
    -e SERVER_NAME=hub.example.com \
    -e MERCURE_TRUSTED_ISSUERS=https://app.example.com \
    -e MERCURE_PUBLISHER_JWT_KEY \
    -e MERCURE_SUBSCRIBER_JWT_KEY \
    -p 80:80 -p 443:443 \
    dunglas/mercure
```

Set `SERVER_NAME` to your public hostname and point its DNS record at the server. Caddy obtains a certificate automatically. The default `localhost` uses a local certificate authority.

Behind a reverse proxy that handles TLS, use HTTP on a private backend network and pin the public audience. See [Reverse proxies](reverse-proxy.md) before exposing this backend:

```console
docker run \
    -e SERVER_NAME=':80' \
    -e MERCURE_TRUSTED_ISSUERS=https://app.example.com \
    -e 'MERCURE_EXTRA_DIRECTIVES=resource_identifier https://hub.example.com/.well-known/mercure' \
    -e MERCURE_PUBLISHER_JWT_KEY='...' \
    -e MERCURE_SUBSCRIBER_JWT_KEY='...' \
    -p 80:80 \
    dunglas/mercure
```

## Mercure Docker development mode

```console
docker run \
    -e MERCURE_EXTRA_DIRECTIVES=playground \
    -p 80:80 -p 443:443 \
    dunglas/mercure
```

The `playground` directive turns on:

- the debug UI at `/.well-known/mercure/debug/`, with a prefilled all-access token,
- anonymous subscribers,
- the playground's echo endpoints,
- a permissive CORS config (`cors_origins *`).

Don't expose this to the internet.

## Compose

Save this as `compose.yaml`, replace the hostnames, then run `docker compose up -d`:

```yaml
services:
  mercure:
    image: dunglas/mercure
    restart: unless-stopped
    environment:
      MERCURE_PUBLISHER_JWT_KEY: ${MERCURE_PUBLISHER_JWT_KEY}
      MERCURE_SUBSCRIBER_JWT_KEY: ${MERCURE_SUBSCRIBER_JWT_KEY}
      MERCURE_TRUSTED_ISSUERS: https://app.example.com
      SERVER_NAME: hub.example.com
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

| Volume    | What's in it                                                   |
| --------- | -------------------------------------------------------------- |
| `/data`   | BoltDB history (`/data/caddy/mercure.db`) and Caddy TLS state. |
| `/config` | Caddy autosaved configuration.                                 |

Persist `/data` to keep history and TLS state across restarts. `/config` stores Caddy's autosaved configuration.

## Mercure Docker healthcheck

The image's built-in health check targets the [transport readiness endpoint](../production/health-monitoring.md) at `localhost:2019/mercure/health/ready`.

For Compose, override or extend it:

```yaml
services:
  mercure:
    # ...
    healthcheck:
      test:
        [
          "CMD",
          "wget",
          "-q",
          "-O",
          "/dev/null",
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
services:
  mercure:
    image: dunglas/mercure
    user: "1000:1000"
    read_only: true
    tmpfs:
      - /tmp
    environment:
      MERCURE_PUBLISHER_JWT_KEY: ${MERCURE_PUBLISHER_JWT_KEY}
      MERCURE_SUBSCRIBER_JWT_KEY: ${MERCURE_SUBSCRIBER_JWT_KEY}
      MERCURE_TRUSTED_ISSUERS: https://app.example.com
      SERVER_NAME: hub.example.com
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - mercure_data:/data
      - mercure_config:/config
```

The volumes must be writable by UID 1000. Use the Compose service to change ownership of the actual project volumes:

```console
docker compose run --rm --user 0 --entrypoint chown mercure -R 1000:1000 /data /config
```

For bind mounts, `chown 1000:1000` the host directory.

## Custom Caddyfile

Ship your own `Caddyfile`:

```yaml
services:
  mercure:
    image: dunglas/mercure
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - mercure_data:/data
      - mercure_config:/config
```

Use a custom Caddyfile for multiple sites, reverse-proxy routes, or additional modules. Available features depend on which modules are compiled into the binary.

## Mercure hub Docker logs

Caddy emits structured logs. Inspect them with `docker logs` or your platform's logging service; Mercure log entries include fields such as `subscriber`, `update`, and `error`.

Bump verbosity with `GLOBAL_OPTIONS=debug` (don't leave it on in prod: it logs update payloads).

## Mercure Docker image variants

- `dunglas/mercure`: Alpine-based, statically linked.
- `dunglas/mercure:<version>`: pin to a specific release.
- `ghcr.io/dunglas/mercure-saas/mercure-saas:1.0`: the licensed Enterprise image with Redis/Valkey, PostgreSQL, Kafka, and Pulsar transports. [Choose a Self-Hosted plan](https://mercure.rocks/pricing), then follow [High availability](../production/high-availability.md).

## Behind a reverse proxy

If you're already running Traefik or NGINX, terminate TLS there and let the hub speak HTTP. See [Reverse proxies](reverse-proxy.md).

## Next steps

- [Configuration](configuration.md): directives and env vars.
- [Kubernetes](kubernetes.md): same image, Helm chart.
- [Health monitoring](../production/health-monitoring.md): probe behavior.
