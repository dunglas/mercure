# Threat model

## What this project does and where untrusted input enters

Mercure is a publish/subscribe protocol over HTTP. The Hub in this repository is a Go library (`github.com/dunglas/mercure`) and a Caddy module (`caddy/`) that serves it. Hubs are exposed directly to the Internet and to browsers.

Treat everything a client sends as untrusted:

- `GET /.well-known/mercure`: subscribers open Server-Sent Events streams. Topic selectors, `lastEventID` / `Last-Event-ID`, and the access token come from the query string, headers, or the `mercureAuthorization` cookie.
- `POST /.well-known/mercure`: publishers send form-encoded updates (topics, `data`, `id`, `type`, `retry`, `private`).
- `GET /.well-known/mercure/subscriptions[/...]`: the subscriptions API.
- `GET /.well-known/oauth-protected-resource/.well-known/mercure`: RFC 9728 metadata derived from the request `Host`.
- JWT access tokens (RFC 9068) carrying `authorization_details`. Claims are attacker-controlled until the signature is verified, and still constrained afterwards.
- JWK Sets fetched from configured `jwks_uri`s.

The hub's operator (Caddyfile, Go options, environment variables) is trusted.

## Components that matter most / least

Highest priority:

- Authorization: `authorization.go`, `authorizationdetails.go`, `matcherclaim.go`, `jwtkeyfunc.go`. Any token accepted with a wrong signature, issuer, audience, algorithm, `typ`, or expiry, any issuer confusion between configured issuers, and any grant broader than the token's `authorization_details` is in scope.
- Private update delivery: a subscriber receiving a `private` update, a subscription event, or a payload for a topic it is not authorized for. See `subscriber.go`, `topicmatcher.go`, `reservedtopic.go`, `subscription.go`.
- Cookie-based authorization and CSRF: `publish_origins` / Origin and Referer checks, CORS in `handler.go`.
- SSE injection: CR, LF or NUL in `id`, `type`, or topics reaching the event stream (`publish.go`, `Update.Validate`).
- Resource exhaustion reachable by an unauthenticated or low-privilege client: unbounded memory, goroutines, or CPU from topic selectors, URL patterns, token claims, history replay (`bolt.go`), or subscription limits bypass (`subscriptionlimits.go`).

Lower priority:

- The playground and debugger (`playground.go`, `debugger.go`, `public/`). They are documented as insecure, development-only, and off by default. Report only issues that affect a hub where they are disabled.
- Compatibility mode for 0.x clients (`protocol_version_compatibility`, `*_deprecated.go`). In scope, but it intentionally accepts legacy token and query formats when the operator enables it.
- `caddy mercure-token` and other CLI helpers run by the operator.

Out of scope: Caddy itself, Go's standard library, and third-party modules, unless Mercure misuses them. `conformance-tests/` and `docs/` are not shipped code.

## How to exercise it

- Unit and integration tests: `go test -race ./...` at the root and in `caddy/`. `GOFLAGS` in the image sets the release build tags.
- Without legacy topic and claim support: `GOFLAGS=-tags=deprecated_transport,nobadger,nomysql,nopgx go test ./...`.
- A running hub: `/usr/local/bin/mercure` is built from `caddy/mercure`. Start it from `/src/caddy/mercure` with:

  ```console
  MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
  MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
  MERCURE_EXTRA_DIRECTIVES='resource_identifier https://localhost/.well-known/mercure' \
  SERVER_NAME=:8080 mercure run --config /src/Caddyfile
  ```

  `mercure mercure-token --dev` mints a token this hub accepts. `docs/concepts/` documents the protocol and token format.

## How you rate severity

- Critical: forging or bypassing access token verification (publishing or subscribing to arbitrary topics without a valid token), or remote code execution.
- High: a valid token granting more than its `authorization_details` allow, private updates or subscription data leaking to an unauthorized subscriber, CSRF publishing through the cookie mechanism, SSE event injection from a publisher into other subscribers' streams.
- Medium: unauthenticated denial of service that one client can trigger with a small number of requests (crash, unbounded memory, CPU amplification), or a bypass of configured subscription limits.
- Low: denial of service that requires a valid publisher token or many connections, information leaks with no direct security impact, issues only reachable with the playground or debugger enabled.

## Anything to leave alone

- Anonymous subscribers (`anonymous`) receiving public updates is the intended behavior.
- The localhost issuer default and demo keys in the sample `Caddyfile` are documented as development settings.
- A hub without `subscription_limits` accepting many connections is a configuration choice, not a bug.
