---
title: "Mercure.rocks hub configuration: Caddyfile and environment variables"
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
    issuer https://example.com {
      publisher {
        jwt {env.MERCURE_PUBLISHER_JWT_KEY}
      }
      subscriber {
        jwt {env.MERCURE_SUBSCRIBER_JWT_KEY}
      }
    }
    resource_identifier https://hub.example.com/.well-known/mercure
    cors_origins        https://example.com
  }

  respond "Not Found" 404
}
```

Each `issuer` binds a trusted issuer (the value accepted in the token `iss` claim, RFC 9068 §4) to its own verification material, so a token is verified only with the key(s) of the issuer it claims. Repeat the block to trust several issuers with distinct keys.

The identifier is the stable identifier of whoever signs the tokens: your app's URL when it signs them itself, or the authorization server's issuer identifier. Add `authorization_server` inside the block to advertise that issuer in the [protected resource metadata](../concepts/discovery.md).

Inside `publisher`/`subscriber`, use `jwt <key> [<algorithm>]` for a shared secret or public key, or `jwks_uri <url> [<algorithm>...]` for a JWK Set. The two are mutually exclusive.

`resource_identifier` is the OAuth 2.0 audience that access tokens must carry in their `aud` claim (see [Authorization](../concepts/authorization.md)). It defaults to `public_url`; set one of them whenever the hub validates tokens.

Caddy provisions a Let's Encrypt certificate for `hub.example.com` automatically. To disable HTTPS (when behind a reverse proxy that terminates TLS), prefix the site address with `http://`:

```caddyfile
# Caddyfile
http://hub.example.com:80 {
  # ...
}
```

Setting the port to 80 also disables HTTPS implicitly.

## Mercure directives

| Directive                                  | Description                                                                                                                               | Default                         |
| ------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------- |
| `issuer <id> { … }`                        | Bind a trusted issuer to its verification material. Repeatable. See [issuer blocks](#issuer-blocks).                                      |                                 |
| `public_url <url>`                         | Canonical hub URL. Resolves relative URL Patterns and topics, and is the default `resource_identifier`.                                   |                                 |
| `resource_identifier <id>`                 | OAuth 2.0 resource identifier (token `aud`). Required when JWT auth is enabled in modern mode. See [Discovery](../concepts/discovery.md). | `public_url`                    |
| `anonymous`                                | Allow subscribers without a token to receive **public** updates.                                                                          | off                             |
| `publish_origins <origin...>`              | Origins allowed to publish (cookie-based auth only).                                                                                      |                                 |
| `cors_origins <origin...>`                 | CORS allowed origins. See [CORS](#cors).                                                                                                  |                                 |
| `cookie_name <name>`                       | Cookie that carries the access token for browser clients. Use a name without the `__Secure-` prefix for plain-HTTP development.           | `__Secure-mercure_access_token` |
| `protocol_version_compatibility <version>` | Accept 0.x behaviors (`7` or `8`). Requires the `deprecated_topic` / `deprecated_claim` build tags. See [Upgrade](../UPGRADE.md).         | off                             |
| `subscriptions`                            | Enable subscription events and the [subscription API](../concepts/active-subscriptions.md).                                               | off                             |
| `heartbeat <duration>`                     | Interval between SSE heartbeat comments. `0s` to disable.                                                                                 | `40s`                           |
| `max_request_body_size <size>`             | Maximum size of publish and QUERY subscribe request bodies (e.g. `512KB`); larger requests get a `413`. `0` delegates to a reverse proxy. | `1MiB`                          |
| `transport <name> [{ <options...> }]`      | Transport configuration. See [Transports](#mercure-hub-transports).                                                                       | `bolt`                          |
| `dispatch_timeout <duration>`              | Max time to dispatch one update to one subscriber. `0s` disables.                                                                         | `5s`                            |
| `write_timeout <duration>`                 | Max duration of a subscriber connection. `0s` disables. See [Rolling updates](../production/rolling-updates.md).                          | `600s`                          |
| `topic_matcher_cache <maxEntries>`         | Cache for topic matcher evaluations. `0` or negative disables it.                                                                         | `100000`                        |
| `subscriber_list_cache_size <maxSize>`     | Subscriber list cache size. `0` for unbounded.                                                                                            | `100000`                        |
| `demo`                                     | Enable the debug UI **and** demo endpoints. Dev only.                                                                                     | off                             |
| `ui`                                       | Enable the debug UI without the demo endpoints.                                                                                           | off                             |

The directives marked dev-only (`demo`, `ui`, `anonymous`) are off by default in production. Don't enable them on a hub that serves real users.

### Issuer blocks

An `issuer` block binds a trusted issuer to its own verification material:

```caddyfile
issuer https://issuer-a.example {
  authorization_server            # advertise in the protected resource metadata
  publisher {
    jwt !ChangeThisSecret! HS256  # shared secret or PEM public key + algorithm
  }
  subscriber {
    jwks_uri https://issuer-a.example/jwks RS256  # JWK Set URL + allowed algorithms
  }
}

issuer https://issuer-b.example {
  publisher  { jwks_uri https://issuer-b.example/jwks }
  subscriber { jwks_uri https://issuer-b.example/jwks }
}
```

| Sub-directive                     | Description                                                                                             |
| --------------------------------- | ------------------------------------------------------------------------------------------------------ |
| `authorization_server`            | Advertise this issuer in the [protected resource metadata](../concepts/discovery.md). Off by default.  |
| `publisher { … }`                 | Verification material for publisher tokens. Omit to reject publishing for this issuer.                  |
| `subscriber { … }`                | Verification material for subscriber tokens. Omit to reject subscribing for this issuer.               |
| `jwt <key> [<algorithm>]`         | Shared secret or PEM public key, plus algorithm (defaults to `HS256`). Supports Caddy placeholders.    |
| `jwks_uri <url> [<algorithm>...]` | JWK Set URL and its allowed algorithms (defaults to the asymmetric allowlist). Accepts `file://` URLs.  |

`jwt` and `jwks_uri` are mutually exclusive within a `publisher`/`subscriber` block.

> [!WARNING]
> The pre-1.0 top-level directives `publisher_jwt`, `subscriber_jwt`, `publisher_jwks_url` and `subscriber_jwks_url` are deprecated. They map to a single implicit issuer and only work in [compatibility mode](../UPGRADE.md); modern mode requires an `issuer` block.

## Mercure hub environment variables

The Docker image and the official Caddyfile read these:

| Variable                        | Description                                                                            | Default     |
| ------------------------------- | -------------------------------------------------------------------------------------- | ----------- |
| `SERVER_NAME`                   | Site address. Use `:80` to bind without a hostname.                                    | `localhost` |
| `MERCURE_PUBLISHER_JWT_KEY`     | Publisher signing key.                                                                 |             |
| `MERCURE_PUBLISHER_JWT_ALG`     | Publisher algorithm.                                                                   | `HS256`     |
| `MERCURE_SUBSCRIBER_JWT_KEY`    | Subscriber signing key.                                                                |             |
| `MERCURE_SUBSCRIBER_JWT_ALG`    | Subscriber algorithm.                                                                  | `HS256`     |
| `MERCURE_RESOURCE_IDENTIFIER`   | Sets `resource_identifier` (the token `aud`).                                          |             |
| `MERCURE_TRUSTED_ISSUERS`       | Sets the `issuer` block identifier (the token `iss`).                                  |             |
| `MERCURE_EXTRA_DIRECTIVES`      | Additional Mercure directives. One per line.                                           |             |
| `GLOBAL_OPTIONS`                | Caddy [global options](https://caddyserver.com/docs/caddyfile/options#global-options). |             |
| `CADDY_EXTRA_CONFIG`            | [Snippets / named routes](https://caddyserver.com/docs/caddyfile/concepts#snippets).   |             |
| `CADDY_SERVER_EXTRA_DIRECTIVES` | Caddyfile directives outside the `mercure` block.                                      |             |
| `MERCURE_LICENSE`               | License key for [Self-Hosted Mercure](https://mercure.rocks/pricing).                  |             |

`MERCURE_EXTRA_DIRECTIVES` is convenient for quick tweaks but **don't put credentials there** (transport passwords, JWKS URLs with tokens). Write a custom Caddyfile and use `{env.MY_SECRET}` for those.

## Mercure hub transports

The transport stores history and (in clustered builds) synchronizes between nodes.

### Bolt transport (default, single-node)

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

| Option              | Description                                                                                 |
| ------------------- | ------------------------------------------------------------------------------------------- |
| `path`              | Path to the BoltDB file. Default: `mercure.db`.                                             |
| `bucket_name`       | Bucket name. Default: `updates`.                                                            |
| `cleanup_frequency` | Probability per publish of running history cleanup. `0` (never) to `1` (always).            |
| `size`              | Maximum number of events to keep. `0` for **unlimited** (default; bound only by disk size). |

The open-source build keeps history forever by default. Set `size` if you want a cap.

### Local transport (no history)

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

Avoid listing the literal `null` origin: browsers send `Origin: null` for sandboxed iframes, `data:` URLs, and local files, so allowlisting it would send credentialed responses to any such opaque context.

If your app and hub run on the same registrable domain (e.g. `example.com` and `hub.example.com`), the hub can be reached without CORS at all by going through a reverse proxy that mounts the hub on the app's origin. See [Reverse proxies](reverse-proxy.md).

## JWT validation via JWKS

When tokens are minted by an external IdP (Keycloak, Cognito, Auth0):

```caddyfile
# JWT validation via JWKS
mercure {
  issuer https://idp.example.com {
    authorization_server
    publisher  { jwks_uri https://idp.example.com/.well-known/jwks.json }
    subscriber { jwks_uri https://idp.example.com/.well-known/jwks.json }
  }
}
```

The hub fetches and caches the keys, validates each token's `kid` against them, and rotates automatically when the IdP rotates. Token issuance stays with the IdP; the hub only verifies.

`jwks_uri` also accepts `file://` URLs, read once at provision time, for keys mounted as files. Append algorithms to pin the allowlist (e.g. `jwks_uri <url> RS256 ES256`); it defaults to the asymmetric algorithms.

## OAuth 2.0 protected resource metadata

When the hub validates tokens, it serves [protected resource metadata](../concepts/discovery.md) (RFC 9728) at `/.well-known/oauth-protected-resource/.well-known/mercure`. Advertise the authorization servers that issue tokens so clients can discover where to obtain one:

```caddyfile
# OAuth 2.0 protected resource metadata
mercure {
  resource_identifier https://hub.example.com/.well-known/mercure
  issuer https://auth.example.com {
    authorization_server
    publisher  { jwks_uri https://auth.example.com/jwks }
    subscriber { jwks_uri https://auth.example.com/jwks }
  }
}
```

## Keeping tokens out of logs

The hub accepts no token in the URL ([RFC 9700](https://www.rfc-editor.org/rfc/rfc9700) forbids it), but misconfigured or legacy clients may still send one there. Redact the known parameter names from access logs; the official Caddyfile does this with a log field filter:

```caddyfile
# Keeping tokens out of logs
log {
  format filter {
    fields {
      request>uri query {
        replace access_token REDACTED
        replace authorization REDACTED
      }
    }
  }
}
```

## RSA / ECDSA keys

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

## Mercure hub health check endpoints

The Caddy admin API (default `localhost:2019`) exposes:

| Endpoint                           | Description                                                 |
| ---------------------------------- | ----------------------------------------------------------- |
| `GET /mercure/health/ready`        | `200` if all transports can serve traffic, `503` otherwise. |
| `GET /mercure/health/live`         | `200` if all transports are fundamentally operational.      |
| `GET /mercure/health/{name}/ready` | Per-hub readiness (when running multiple).                  |
| `GET /mercure/health/{name}/live`  | Per-hub liveness.                                           |

The endpoints bind to `localhost` for security. Probes from outside the container should use `kubectl exec` or `docker exec` (see [Health monitoring](../production/health-monitoring.md)). Binding the admin API to `0.0.0.0:2019` works but exposes `/stop` and `/load` to the pod network. Almost never what you want.

## Mercure hub performance tuning

A few knobs that move the needle:

- `dispatch_timeout`: too low and slow subscribers get cut off; too high and a stuck dispatch ties up resources. The 5s default is a reasonable starting point.
- `write_timeout`: controls how often each subscriber rotates its connection in steady state. Higher values mean fewer reconnects but worse drain pacing on shutdown. See [Rolling updates](../production/rolling-updates.md).
- `topic_matcher_cache` and `subscriber_list_cache_size`: increase if your hub has many distinct matchers and you see CPU spent in matcher evaluation. Decrease if memory is tight.
- File descriptors: every subscriber takes one. `ulimit -n 100000` on the host (or the equivalent in your orchestrator) for high-fanout hubs.

[Load testing](../production/load-testing.md) and [Debugging](../production/debugging.md) cover the rest.

## Mercure hub configuration reload

Caddy hot-reloads on signal: `kill -USR1 <pid>` or `caddy reload`. Active SSE connections are preserved across reloads as long as the listening sockets don't change.

## Mercure hub runtime introspection

The Caddy admin API also exposes:

- `/config/`: the current effective config (JSON).
- `/metrics`: Prometheus metrics (when `metrics` is in `GLOBAL_OPTIONS`).
- `/debug/pprof/`: Go profiler endpoints (when `debug` is in `GLOBAL_OPTIONS`). See [Debugging](../production/debugging.md).
