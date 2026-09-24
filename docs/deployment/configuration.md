---
title: "Mercure.rocks hub configuration: Caddyfile and environment variables"
description: "Configure the Mercure.rocks Hub with Caddyfile directives, environment variables, transports, CORS, and JWKS validation."
---

# Mercure configuration

The Mercure.rocks Hub is a [Caddy](https://caddyserver.com/) build with the Mercure module. Anything the [Caddy docs](https://caddyserver.com/docs/) describe also applies to this binary.

The bundled `Caddyfile` supports environment variables for common settings. For more control, write a custom Caddyfile or use [Caddy JSON configuration](https://caddyserver.com/docs/json/).

## Minimal Caddyfile

```caddyfile
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

The algorithm defaults to `HS256` only for a raw shared secret. A PEM-encoded key must state its algorithm, and that algorithm must not be an HMAC one: the hub refuses to start otherwise, because verifying with `HS*` would use the public key as the shared secret and let anyone holding it forge tokens.

`resource_identifier` is the OAuth 2.0 audience that access tokens must carry in their `aud` claim (see [Authorization](../concepts/authorization.md)). Leave it unset and the hub derives it from each request (the public URL the client contacted), so a hub reachable through several domains needs no configuration; set it only to pin one canonical audience shared across every domain.

`resource_identifier` and `public_urls` answer different questions and are independent: `resource_identifier` sets the token audience, while `public_urls` restricts which origins the hub answers on (rejecting others with `421`). A single `resource_identifier` is what lets one token work across several public URLs, since a per-request-derived audience is specific to the host the client contacted.

Caddy provisions a Let's Encrypt certificate for `hub.example.com` automatically. To disable HTTPS (when behind a reverse proxy that terminates TLS), prefix the site address with `http://`:

```caddyfile
http://hub.example.com:80 {
  # ...
}
```

Setting the port to 80 also disables HTTPS implicitly.

## Mercure directives

| Directive                                  | Description                                                                                                                                                 | Default                         |
| ------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------- |
| `issuer <id> { … }`                        | Bind a trusted issuer to its verification material. Repeatable. See [issuer blocks](#issuer-blocks).                                                        |                                 |
| `resource_identifier <id>`                 | Pin the OAuth 2.0 resource identifier (token `aud`); unset, it is derived per request. See [Discovery](../concepts/discovery.md).                           | derived per request             |
| `public_urls <url...>`                     | Public URLs the hub answers on (scheme pinned); an unlisted origin gets `421 Misdirected Request`. Set it on a catch-all site.                              | site host matching              |
| `anonymous`                                | Allow subscribers without a token to receive **public** updates.                                                                                            | off                             |
| `publish_origins <origin...>`              | Origins allowed to publish (cookie-based auth only).                                                                                                        |                                 |
| `cors_origins <origin...>`                 | CORS allowed origins. See [CORS](#cors).                                                                                                                    |                                 |
| `cookie_name <name>`                       | Cookie that carries the access token for browser clients. Use a prefix-less name only for local HTTP development.                                           | `__Secure-mercure_access_token` |
| `protocol_version_compatibility <version>` | Accept 0.x behaviors (`7` or `8`). Requires the `deprecated_topic` / `deprecated_claim` build tags. See [Upgrade](../UPGRADE.md).                           | off                             |
| `subscriptions`                            | Enable subscription events and the [subscription API](../concepts/active-subscriptions.md).                                                                 | off                             |
| `heartbeat <duration>`                     | Interval between SSE heartbeat comments. `0s` to disable.                                                                                                   | `40s`                           |
| `max_request_body_size <size>`             | Maximum size of publish and QUERY subscribe request bodies (e.g. `512KB`); larger requests get a `413`. `0` delegates to a reverse proxy.                   | `1MiB`                          |
| `transport <name> [{ <options...> }]`      | Transport configuration. See [Transports](#mercure-hub-transports).                                                                                         | `bolt`                          |
| `dispatch_timeout <duration>`              | Max time to dispatch one update to one subscriber. `0s` disables.                                                                                           | `5s`                            |
| `write_timeout <duration>`                 | Max duration of a subscriber connection. `0s` disables. See [Rolling updates](../production/rolling-updates.md).                                            | `600s`                          |
| `topic_matcher_cache <maxEntries>`         | Cache for topic matcher evaluations, sized in entries of ~100 B (longer topics count for more). `0` or negative disables it.                                | `100000`                        |
| `subscriber_list_cache_size <maxSize>`     | Subscriber list cache size. `0` for unbounded.                                                                                                              | `100000`                        |
| `debugger`                                 | Serve the debugger UI at `/.well-known/mercure/debug/` (no token, no playground endpoints). Safe in production.                                             | off                             |
| `playground`                               | Enable `debugger` **and** the insecure playground: the `/playground/` discovery endpoints, and a hub-minted all-access token prefilled in the UI. Dev only. | off                             |

`debugger` serves a browser client that uses the token you provide. `playground` also creates an all-access token and enables permissive defaults. Use `playground` only for development. For a protected hub, generate a scoped token with [`caddy mercure-token`](../concepts/authorization.md#minting-a-token).

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
  publisher {
    jwks_uri https://issuer-b.example/jwks
  }
  subscriber {
    jwks_uri https://issuer-b.example/jwks
  }
}
```

| Sub-directive                     | Description                                                                                            |
| --------------------------------- | ------------------------------------------------------------------------------------------------------ |
| `authorization_server`            | Advertise this issuer in the [protected resource metadata](../concepts/discovery.md). Off by default.  |
| `publisher { … }`                 | Verification material for publisher tokens. Omit to reject publishing for this issuer.                 |
| `subscriber { … }`                | Verification material for subscriber tokens. Omit to reject subscribing for this issuer.               |
| `jwt <key> [<algorithm>]`         | Shared secret or PEM public key, plus algorithm. A PEM key must set a non-HMAC one (see above).        |
| `jwks_uri <url> [<algorithm>...]` | JWK Set URL and its allowed algorithms (defaults to the asymmetric allowlist). Accepts `file://` URLs. |

`jwt` and `jwks_uri` are mutually exclusive within a `publisher`/`subscriber` block.

> [!WARNING]
> The pre-1.0 top-level directives `publisher_jwt`, `subscriber_jwt`, `publisher_jwks_url` and `subscriber_jwks_url` are deprecated. They map to a single implicit issuer and only work in [compatibility mode](../UPGRADE.md); modern mode requires an `issuer` block.

## Mercure hub environment variables

The Docker image and the official Caddyfile read these:

| Variable                        | Description                                                                            | Default             |
| ------------------------------- | -------------------------------------------------------------------------------------- | ------------------- |
| `SERVER_NAME`                   | Site address. Use `:80` to bind without a hostname.                                    | `localhost`         |
| `MERCURE_PUBLISHER_JWT_KEY`     | Publisher verification secret or public key.                                           |                     |
| `MERCURE_PUBLISHER_JWT_ALG`     | Publisher algorithm.                                                                   | `HS256`             |
| `MERCURE_SUBSCRIBER_JWT_KEY`    | Subscriber verification secret or public key.                                          |                     |
| `MERCURE_SUBSCRIBER_JWT_ALG`    | Subscriber algorithm.                                                                  | `HS256`             |
| `MERCURE_TRUSTED_ISSUERS`       | Sets the `issuer` block identifier (the token `iss`).                                  | `https://localhost` |
| `MERCURE_EXTRA_DIRECTIVES`      | Additional Mercure directives. One per line.                                           |                     |
| `GLOBAL_OPTIONS`                | Caddy [global options](https://caddyserver.com/docs/caddyfile/options#global-options). |                     |
| `CADDY_EXTRA_CONFIG`            | [Snippets / named routes](https://caddyserver.com/docs/caddyfile/concepts#snippets).   |                     |
| `CADDY_SERVER_EXTRA_DIRECTIVES` | Caddyfile directives outside the `mercure` block.                                      |                     |
| `MERCURE_LICENSE`               | License key for [Self-Hosted Mercure](https://mercure.rocks/pricing).                  |                     |

Use your deployment's secret store for credentials. The JWT directives accept runtime `{env.MY_SECRET}` placeholders. Placeholder support depends on the module: follow the [Enterprise transport examples](../production/high-availability.md#self-hosted-transports) for shared-backend credentials.

## Mercure hub transports

The transport stores history and (in clustered builds) synchronizes between nodes.

### Bolt transport (default, single-node)

```caddyfile
mercure {
  transport bolt {
    path /data/mercure.db
    size 0
    cleanup_frequency 0.3
  }
  # ...
}
```

| Option              | Description                                                                                                                   |
| ------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `path`              | Path to the BoltDB file. Default: `mercure.db` in Caddy's application data directory (`/data/caddy/mercure.db` in the image). |
| `bucket_name`       | Bucket name. Default: `updates`.                                                                                              |
| `cleanup_frequency` | Probability per publish of running history cleanup; set explicitly when using `size`. `0` (never) to `1` (always).            |
| `size`              | Retention target; cleanup is probabilistic. `0` for **unlimited** (default; bound only by disk size).                         |

With `size 0`, BoltDB does not prune history automatically. Set both `size` and a positive `cleanup_frequency` to enable retention cleanup.

### Local transport (no history)

`transport local` disables history entirely. Use it when reconnect replay isn't needed and you want the lowest possible memory footprint.

### Shared transports

[Mercure Enterprise](https://mercure.rocks/pricing) includes Redis/Valkey, PostgreSQL, Kafka, and Pulsar transports to distribute updates and share history across hub instances. See [transport configuration examples](../production/high-availability.md#self-hosted-transports). Prefer a managed hub? [Mercure Cloud](https://mercure.rocks/pricing) handles the infrastructure for you.

## CORS

If the page that opens the SSE connection is on a different origin than the hub, you must list it in `cors_origins`:

```caddyfile
mercure {
  cors_origins https://app.example.com https://admin.example.com
}
```

`cors_origins *` allows cross-origin requests without cookies. For `EventSource` with `withCredentials: true`, list explicit origins. Sending an `Authorization` header with `fetch` also requires a successful CORS preflight.

Avoid listing the literal `null` origin: browsers send `Origin: null` for sandboxed iframes, `data:` URLs, and local files, so allowlisting it would send credentialed responses to any such opaque context.

To avoid CORS, expose the hub on the **same origin** as your application: the same scheme, hostname, and port. Sibling subdomains are different origins. See [Reverse proxies](reverse-proxy.md).

## JWT validation via JWKS

When tokens are minted by an external IdP (Keycloak, Cognito, Auth0):

```caddyfile
mercure {
  issuer https://idp.example.com {
    authorization_server
    publisher {
      jwks_uri https://idp.example.com/.well-known/jwks.json
    }
    subscriber {
      jwks_uri https://idp.example.com/.well-known/jwks.json
    }
  }
}
```

The hub fetches and caches the keys, validates each token's `kid` against them, and rotates automatically when the IdP rotates. Token issuance stays with the IdP; the hub only verifies.

`jwks_uri` also accepts `file://` URLs, read once at provision time, for keys mounted as files. Append algorithms to pin the allowlist (e.g. `jwks_uri <url> RS256 ES256`); it defaults to the asymmetric algorithms.

## OAuth 2.0 protected resource metadata

When the hub validates tokens, it serves [protected resource metadata](../concepts/discovery.md) (RFC 9728) at `/.well-known/oauth-protected-resource/.well-known/mercure`. Advertise the authorization servers that issue tokens so clients can discover where to obtain one:

```caddyfile
mercure {
  resource_identifier https://hub.example.com/.well-known/mercure
  issuer https://auth.example.com {
    authorization_server
    publisher {
      jwks_uri https://auth.example.com/jwks
    }
    subscriber {
      jwks_uri https://auth.example.com/jwks
    }
  }
}
```

## Keeping tokens out of logs

Modern clients must send tokens in a header or cookie. The hub accepts the old `authorization` query parameter only in [compatibility mode](../UPGRADE.md#compatibility-mode). Redact it from access logs while migrating legacy clients:

```caddyfile
log {
  format filter {
    fields {
      request>uri query {
        replace authorization REDACTED
      }
    }
  }
}
```

## RSA / ECDSA keys

```console
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:4096 -out publisher.key
openssl pkey -in publisher.key -pubout -out publisher.key.pub
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:4096 -out subscriber.key
openssl pkey -in subscriber.key -pubout -out subscriber.key.pub
```

Start the hub with the public key for verification and the algorithm:

```console
MERCURE_PUBLISHER_JWT_KEY="$(cat publisher.key.pub)" \
MERCURE_PUBLISHER_JWT_ALG=RS256 \
MERCURE_SUBSCRIBER_JWT_KEY="$(cat subscriber.key.pub)" \
MERCURE_SUBSCRIBER_JWT_ALG=RS256 \
./mercure run --config Caddyfile
```

## Mercure hub health check endpoints

The Caddy admin API (default `localhost:2019`) exposes:

| Endpoint                           | Description                                                 |
| ---------------------------------- | ----------------------------------------------------------- |
| `GET /mercure/health/ready`        | `200` if all transports can serve traffic, `503` otherwise. |
| `GET /mercure/health/live`         | `200` if all transports are fundamentally operational.      |
| `GET /mercure/health/{name}/ready` | Per-hub readiness (when running multiple).                  |
| `GET /mercure/health/{name}/live`  | Per-hub liveness.                                           |

The endpoints bind to `localhost` for security. Probes from outside the container should use `kubectl exec` or `docker exec` (see [Health monitoring](../production/health-monitoring.md)). Binding the admin API to `0.0.0.0:2019` works but exposes `/stop` and `/load` to the pod network. Restrict access if you use that configuration.

## Mercure hub performance tuning

Tune these settings using measurements from your workload:

- `dispatch_timeout`: too low and slow subscribers get cut off; too high and a stuck dispatch ties up resources. The 5s default is a reasonable starting point.
- `write_timeout`: controls how often each subscriber rotates its connection in steady state. Higher values mean fewer reconnects but worse drain pacing on shutdown. See [Rolling updates](../production/rolling-updates.md).
- `topic_matcher_cache` and `subscriber_list_cache_size`: increase if your hub has many distinct matchers and you see CPU spent in matcher evaluation. Decrease if memory is tight.
- File descriptors: each TCP connection consumes one; HTTP/2 streams can share a connection. `ulimit -n 100000` on the host (or the equivalent in your orchestrator) for high-fanout hubs.

[Load testing](../production/load-testing.md) and [Debugging](../production/debugging.md) cover the rest.

## Mercure hub configuration reload

Use `caddy reload --config /etc/caddy/Caddyfile` to apply configuration changes. Active subscriptions may drain and reconnect; see [Rolling updates](../production/rolling-updates.md#graceful-mercure-hub-configuration-reloads).

On Unix, you can also reload the configuration file with `SIGUSR1`. Set `MERCURE_PID` to the hub process ID:

```console
kill -USR1 "$MERCURE_PID"
```

This works when the hub was started with `run` and a configuration file, without `--resume`. Switching to API-based configuration can disable signal reloads; see [Caddy's signal rules](https://caddyserver.com/docs/command-line#signals).

## Mercure hub runtime introspection

The Caddy admin API also exposes:

- `/config/`: the current effective config (JSON).
- `/metrics`: Prometheus metrics (when `metrics` is in `GLOBAL_OPTIONS`).
- `/debug/pprof/`: Go profiler endpoints on the admin API. See [Debugging](../production/debugging.md).
