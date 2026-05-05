---
title: "Mercure.rocks Hub Configuration: Caddyfile and Environment Variables"
description: "Configure the Mercure.rocks Hub with Caddyfile directives, environment variables, transports, CORS, and JWKS validation."
---

# Configuration

The Mercure.rocks Hub is a [Caddy](https://caddyserver.com/) build with the Mercure module. Anything the [Caddy docs](https://caddyserver.com/docs/) describe also applies to this binary.

The most idiomatic way to configure it is a [`Caddyfile`](https://caddyserver.com/docs/quick-starts/caddyfile). Other formats (JSON, the admin API, env-var-driven config) work too; Mercure ships pre-wired for env vars in the official Docker image.

## Minimal Caddyfile

```caddyfile
# Caddyfile
hub.example.com {
  mercure {
    publisher_jwt  {env.MERCURE_PUBLISHER_JWT_KEY}
    subscriber_jwt {env.MERCURE_SUBSCRIBER_JWT_KEY}
    cors_origins   https://example.com
  }

  respond "Not Found" 404
}
```

Caddy provisions a Let's Encrypt certificate for `hub.example.com` automatically. To disable HTTPS (when behind a reverse proxy that terminates TLS), prefix the site address with `http://`:

```caddyfile
# Caddyfile
http://hub.example.com:80 {
  # ...
}
```

Setting the port to 80 also disables HTTPS implicitly.

## Mercure Directives

| Directive                                      | Description                                                                                                               | Default                |
| ---------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- | ---------------------- |
| `publisher_jwt <key> [<algorithm>]`            | JWT key + algorithm for publishers. Supports [Caddy placeholders](https://caddyserver.com/docs/conventions#placeholders). |                        |
| `subscriber_jwt <key> [<algorithm>]`           | JWT key + algorithm for subscribers.                                                                                      |                        |
| `publisher_jwks_url <url>`                     | JWK Set URL for publisher token validation. Takes precedence over `publisher_jwt`.                                        |                        |
| `subscriber_jwks_url <url>`                    | JWK Set URL for subscriber token validation.                                                                              |                        |
| `anonymous`                                    | Allow subscribers without a JWT to receive **public** updates.                                                            | off                    |
| `publish_origins <origin...>`                  | Origins allowed to publish (cookie-based auth only).                                                                      |                        |
| `cors_origins <origin...>`                     | CORS allowed origins. See [CORS](#cors).                                                                                  |                        |
| `cookie_name <name>`                           | Cookie that carries the JWT for browser clients.                                                                          | `mercureAuthorization` |
| `subscriptions`                                | Enable subscription events and the [subscription API](../concepts/active-subscriptions.md).                               | off                    |
| `heartbeat <duration>`                         | Interval between SSE heartbeat comments. `0s` to disable.                                                                 | `40s`                  |
| `transport <name> [{ <options...> }]`          | Transport configuration. See [Transports](#transports).                                                                   | `bolt`                 |
| `dispatch_timeout <duration>`                  | Max time to dispatch one update to one subscriber. `0s` disables.                                                         | `5s`                   |
| `write_timeout <duration>`                     | Max duration of a subscriber connection. `0s` disables. See [Rolling updates](../production/rolling-updates.md).          | `600s`                 |
| `topic_selector_cache <maxEntries> [<shards>]` | Cache for matcher evaluations. `-1` to disable, `0` for unbounded.                                                        | `10000 256`            |
| `subscriber_list_cache_size <maxSize>`         | Subscriber list cache size. `0` for unbounded.                                                                            | `100000`               |
| `demo`                                         | Enable the debug UI **and** demo endpoints. Dev only.                                                                     | off                    |
| `ui`                                           | Enable the debug UI without the demo endpoints.                                                                           | off                    |

The directives marked dev-only (`demo`, `ui`, `anonymous`) are off by default in production. Don't enable them on a hub that serves real users.

## Mercure Hub Environment Variables

The Docker image and the official Caddyfile read these:

| Variable                        | Description                                                                            | Default     |
| ------------------------------- | -------------------------------------------------------------------------------------- | ----------- |
| `SERVER_NAME`                   | Site address. Use `:80` to bind without a hostname.                                    | `localhost` |
| `MERCURE_PUBLISHER_JWT_KEY`     | Publisher signing key.                                                                 |             |
| `MERCURE_PUBLISHER_JWT_ALG`     | Publisher algorithm.                                                                   | `HS256`     |
| `MERCURE_SUBSCRIBER_JWT_KEY`    | Subscriber signing key.                                                                |             |
| `MERCURE_SUBSCRIBER_JWT_ALG`    | Subscriber algorithm.                                                                  | `HS256`     |
| `MERCURE_EXTRA_DIRECTIVES`      | Additional Mercure directives. One per line.                                           |             |
| `GLOBAL_OPTIONS`                | Caddy [global options](https://caddyserver.com/docs/caddyfile/options#global-options). |             |
| `CADDY_EXTRA_CONFIG`            | [Snippets / named routes](https://caddyserver.com/docs/caddyfile/concepts#snippets).   |             |
| `CADDY_SERVER_EXTRA_DIRECTIVES` | Caddyfile directives outside the `mercure` block.                                      |             |
| `MERCURE_LICENSE`               | License key for [Self-Hosted Mercure](https://mercure.rocks/pricing).                  |             |

`MERCURE_EXTRA_DIRECTIVES` is convenient for quick tweaks but **don't put credentials there** (transport passwords, JWKS URLs with tokens). Write a custom Caddyfile and use `{env.MY_SECRET}` for those.

## Mercure Hub Transports

The transport stores history and (in clustered builds) synchronizes between nodes.

### Bolt Transport (Default, Single-Node)

```caddyfile
# Bolt transport (default, single-node)
mercure {
  transport bolt {
    path /data/mercure.db
    size 0
    cleanup_frequency 0.3
  }
  # ...
}
```

| Option              | Description                                                                                  |
| ------------------- | -------------------------------------------------------------------------------------------- |
| `path`              | Path to the BoltDB file. Default: `mercure.db`.                                              |
| `bucket_name`       | Bucket name. Default: `updates`.                                                             |
| `cleanup_frequency` | Probability per publish of running history cleanup. `0` (never) to `1` (always).             |
| `size`              | Maximum number of events to keep. `0` for **unlimited** (default; bound only by disk size). |

The open-source build keeps history forever by default. Set `size` if you want a cap.

### Local Transport (No History)

`transport local` disables history entirely. Use it when reconnect replay isn't needed and you want the lowest possible memory footprint.

### Redis / Postgres / Kafka / Pulsar

These ship with [Self-Hosted Mercure](../production/high-availability.md). They enable multi-node deployments and queryable history.

> **Pro tip.** The open-source hub runs on a single node. For redundancy across nodes, low-latency multi-region deploys, or storing events in Redis or Postgres for SQL-backed queries, [Self-Hosted Mercure](https://mercure.rocks/pricing) ships those transports starting at €1,500/year.

## CORS

If the page that opens the SSE connection is on a different origin than the hub, you must list it in `cors_origins`:

```caddyfile
# CORS
mercure {
  cors_origins https://app.example.com https://admin.example.com
}
```

`*` is allowed only if the hub is fully anonymous (no JWT, no cookie). Browsers refuse credentialed requests from a wildcard origin.

If your app and hub run on the same registrable domain (e.g. `example.com` and `hub.example.com`), the hub can be reached without CORS at all by going through a reverse proxy that mounts the hub on the app's origin. See [Reverse proxies](reverse-proxy.md).

## JWT Validation via JWKS

When tokens are minted by an external IdP (Keycloak, Cognito, Auth0):

```caddyfile
# JWT validation via JWKS
mercure {
  publisher_jwks_url https://idp.example.com/.well-known/jwks.json
  subscriber_jwks_url https://idp.example.com/.well-known/jwks.json
}
```

The hub fetches and caches the keys, validates each token's `kid` against them, and rotates automatically when the IdP rotates. Token issuance stays with the IdP; the hub only verifies.

## RSA / ECDSA Keys

```console
# RSA / ECDSA keys
ssh-keygen -t rsa -b 4096 -m PEM -f publisher.key
openssl rsa -in publisher.key -pubout -outform PEM -out publisher.key.pub
```

Start the hub with the public key for verification and the algorithm:

```console
# RSA / ECDSA keys
MERCURE_PUBLISHER_JWT_KEY="$(cat publisher.key.pub)" \
MERCURE_PUBLISHER_JWT_ALG=RS256 \
MERCURE_SUBSCRIBER_JWT_KEY="$(cat subscriber.key.pub)" \
MERCURE_SUBSCRIBER_JWT_ALG=RS256 \
./mercure run
```

## Mercure Hub Health Check Endpoints

The Caddy admin API (default `localhost:2019`) exposes:

| Endpoint                           | Description                                                 |
| ---------------------------------- | ----------------------------------------------------------- |
| `GET /mercure/health/ready`        | `200` if all transports can serve traffic, `503` otherwise. |
| `GET /mercure/health/live`         | `200` if all transports are fundamentally operational.      |
| `GET /mercure/health/{name}/ready` | Per-hub readiness (when running multiple).                  |
| `GET /mercure/health/{name}/live`  | Per-hub liveness.                                           |

The endpoints bind to `localhost` for security. Probes from outside the container should use `kubectl exec` or `docker exec` (see [Health monitoring](../production/health-monitoring.md)). Binding the admin API to `0.0.0.0:2019` works but exposes `/stop` and `/load` to the pod network. Almost never what you want.

## Mercure Hub Performance Tuning

A few knobs that move the needle:

- `dispatch_timeout`: too low and slow subscribers get cut off; too high and a stuck dispatch ties up resources. The 5s default is a reasonable starting point.
- `write_timeout`: controls how often each subscriber rotates its connection in steady state. Higher values mean fewer reconnects but worse drain pacing on shutdown. See [Rolling updates](../production/rolling-updates.md).
- `topic_selector_cache` and `subscriber_list_cache_size`: increase if your hub has many distinct matchers and you see CPU spent in matcher evaluation. Decrease if memory is tight.
- File descriptors: every subscriber takes one. `ulimit -n 100000` on the host (or the equivalent in your orchestrator) for high-fanout hubs.

[Load testing](../production/load-testing.md) and [Debugging](../production/debugging.md) cover the rest.

## Mercure Hub Configuration Reload

Caddy hot-reloads on signal: `kill -USR1 <pid>` or `caddy reload`. Active SSE connections are preserved across reloads as long as the listening sockets don't change.

## Mercure Hub Runtime Introspection

The Caddy admin API also exposes:

- `/config/`: the current effective config (JSON).
- `/metrics`: Prometheus metrics (when `metrics` is in `GLOBAL_OPTIONS`).
- `/debug/pprof/`: Go profiler endpoints (when `debug` is in `GLOBAL_OPTIONS`). See [Debugging](../production/debugging.md).
