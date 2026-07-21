---
title: "Mercure 1.0 upgrade guide"
description: "Step-by-step migration from Mercure 0.x to 1.0: two matcher types, OAuth 2.0 access tokens with authorization_details, RFC 6750 errors, and the new subscription event format."
---

# Upgrade guide

## 1.0 (from 0.x)

The 1.0 release aligns the hub with the standards-based specification: two matcher types and an OAuth 2.0 authorization model. It is a **breaking change** for subscribers, publishers, and token issuers.

If you only run the hub and don't author clients or mint tokens, the upgrade is a config change. If you do, plan a synchronized cutover of the hub and the clients that talk to it. A hub built with the `deprecated_topic` and `deprecated_claim` tags can run `protocol_version_compatibility 8` to keep accepting 0.x clients during the transition (see [Compatibility mode](#compatibility-mode)).

### What changed at a glance

| Area                      | 0.x                                                                | 1.0                                                                            |
| ------------------------- | ------------------------------------------------------------------ | ------------------------------------------------------------------------------ |
| Matcher types             | URI Template, string, plus exploratory types                       | `exact` and `urlpattern` only                                                  |
| Subscribe query parameter | `topic=<pattern>` (URI Template or string)                         | `match=<exact>` or `match_urlpattern=<pattern>` (case-sensitive)               |
| Templating language       | URI Templates ([RFC 6570](https://www.rfc-editor.org/rfc/rfc6570)) | URL Patterns ([WHATWG](https://urlpattern.spec.whatwg.org))                    |
| Topics per update         | Canonical + alternates                                             | Exactly one                                                                    |
| Token                     | bespoke `mercure` JWT claim                                        | OAuth 2.0 access token: `typ: at+jwt`, `iss`, `aud`, `authorization_details`   |
| Authorization             | `mercure.publish` / `mercure.subscribe` string arrays              | `authorization_details` entries with the Mercure `type` URI (see below)        |
| Token in query / cookie   | `authorization` param, `mercureAuthorization` cookie               | `__Secure-mercure_access_token` cookie; no query parameter (RFC 9700)          |
| Auth errors               | `401` / silent drop                                                | RFC 6750: `401 invalid_token`, `403 insufficient_scope`, `400 invalid_request` |
| Subscription event topic  | `/.well-known/mercure/subscriptions/{topic}/{subscriber}`          | `/.well-known/mercure/subscriptions/{match_type}/{match}/{subscriber}`         |

### Migrate your subscribers

The query parameter changes from `topic=` to `match=` (exact) or `match_urlpattern=` (templated):

```javascript
// Before (0.x)
url.searchParams.append("topic", "https://example.com/books/1");
url.searchParams.append("topic", "https://example.com/books/{id}");

// After (1.0)
url.searchParams.append("match", "https://example.com/books/1");
url.searchParams.append("match_urlpattern", "https://example.com/books/:id");
```

- The exact-match parameter is `match` (explicit spelling: `match_exact`); the templated one is `match_urlpattern`, using [URL Pattern](https://urlpattern.spec.whatwg.org) syntax (`:id`, not `{id}`).
- Parameter names are **case-sensitive**. Any other name under the `match` prefix is rejected with `400`, so typos fail loudly.
- The `Regexp`, `CEL`, and `URI Template` matcher types are gone. Rewrite `Regexp`/CEL filters as URL Patterns or exact topics. URI Templates survive only on a hub built with `deprecated_topic` running `protocol_version_compatibility 8`.

### Migrate your tokens

The bespoke `mercure` claim is replaced by a standard OAuth 2.0 [JWT access token](concepts/authorization.md): a `typ: at+jwt` header, an `iss` claim matching one of the hub's trusted issuers, an `aud` claim holding the hub's resource identifier, a required `exp`, and an `authorization_details` array. [RFC 9068](https://www.rfc-editor.org/rfc/rfc9068) also requires issuers to populate `sub`, `client_id`, `iat`, and `jti`.

```jsonc
// Before (0.x)
{
  "mercure": {
    "publish": ["*"],
    "subscribe": [
      "https://example.com/users/42",
      "https://example.com/books/{id}",
    ],
  },
}
```

```jsonc
// After (1.0) — header { "alg": "...", "typ": "at+jwt" }
{
  "iss": "https://example.com",
  "aud": "https://hub.example.com/.well-known/mercure",
  "exp": 4102444800,
  "authorization_details": [
    {
      "type": "https://mercure.rocks/authorization-detail",
      "actions": ["publish"],
      "topics": [{ "match": "*" }],
    },
    {
      "type": "https://mercure.rocks/authorization-detail",
      "actions": ["subscribe"],
      "topics": [
        { "match": "https://example.com/users/42" },
        {
          "match": "https://example.com/books/:id",
          "match_type": "urlpattern",
        },
      ],
    },
  ],
}
```

Rules:

- Each entry is `{ "type": "https://mercure.rocks/authorization-detail", "actions": [...], "topics": [...] }`. `actions` is a non-empty subset of `["publish", "subscribe"]`; `topics` is a non-empty array of `{ "match", "match_type"? }` objects (bare strings are rejected).
- `match_type` is **case-sensitive** and defaults to `exact`. The reserved `{ "match": "*" }` matches every topic.
- A `subscribe` entry may carry a `payload`; the old top-level `mercure.payload` is gone. See [subscriber payloads](concepts/authorization.md#subscriber-payloads).
- One invalid Mercure detail rejects the whole token with `401 invalid_token`.

### Migrate token presentation

- The cookie default name changes from `mercureAuthorization` to `__Secure-mercure_access_token` (override with `cookie_name`; use a prefix-less name for plain-HTTP development).
- The `authorization` query parameter is gone and has no replacement: [RFC 9700](https://www.rfc-editor.org/rfc/rfc9700) forbids access tokens in URLs. Browsers that can't set headers use the cookie; everything else (including `fetch()` with a readable stream) uses the `Authorization` header.
- The `Authorization: Bearer` header is unchanged and takes precedence over the cookie.

### Migrate to RFC 6750 errors

Authorization failures now follow [RFC 6750](https://www.rfc-editor.org/rfc/rfc6750):

- No token where one is required -> `401` with a bare `WWW-Authenticate: Bearer` challenge and a `resource_metadata` parameter.
- Invalid token -> `401` `error="invalid_token"`.
- Valid token without a grant for the action on the topic -> `403` `error="insufficient_scope"` (previously `401` or a silent drop).
- Malformed request -> `400` `error="invalid_request"`.

### Migrate the subscription API and events

| Before                                                    | After                                                                  |
| --------------------------------------------------------- | ---------------------------------------------------------------------- |
| `/.well-known/mercure/subscriptions/<topic>/<subscriber>` | `/.well-known/mercure/subscriptions/<match_type>/<match>/<subscriber>` |
| `"topic": "https://..."` in the JSON-LD                   | `"match": "https://..."` and `"match_type": "urlpattern"`              |

`<match_type>`, `<match>`, and `<subscriber>` must be percent-encoded. The `mercure.subscriber` claim is gone: the hub assigns the subscriber identifier. See [Active subscriptions](concepts/active-subscriptions.md).

### Find-and-replace checklist

- `?topic=` / `&topic=` in subscriber URLs -> `match=` (or `match_urlpattern=` if templated)
- URI Template syntax in subscribe URLs (`{id}`) -> URL Pattern syntax (`:id`)
- `Regexp` / CEL subscribe filters -> URL Patterns or exact topics
- `"mercure": { "publish": [...] }` in issuer code -> `authorization_details` with `actions: ["publish"]`
- `"mercure": { "subscribe": [...] }` -> `authorization_details` with `actions: ["subscribe"]`
- `mercureAuthorization` cookie -> `mercure_access_token`; `authorization=` query param -> `Authorization` header or cookie (no query parameter)
- A second `topic=` on a publish request -> publish to one topic; scope per-user access in the token
- Hardcoded `subscriptions/{topic}/{subscriber}` paths -> add the `{match_type}` segment

- JSON-LD subscription documents (`application/ld+json`, `@context`) -> plain JSON served as `application/json`
- `type` values lowercased: `Subscription` -> `subscription`, `Subscriptions` -> `subscriptions`
- Subscription events now carry the SSE `event: mercure` field (route them with `addEventListener("mercure", ...)`); publishing an update whose `type` is `mercure` is rejected with a `400 Bad Request`

### Hub configuration changes

- Set `resource_identifier` (or `public_url`) to the audience your tokens carry; it's required when JWT auth is enabled in modern mode. The official Caddyfile defaults it to `https://localhost/.well-known/mercure`.
- Declare your token issuer with an `issuer <id> { ... }` block binding the `iss` value your tokens carry to its `publisher`/`subscriber` verifier (`jwt` or `jwks_uri`); it's required when JWT auth is enabled in modern mode. Add `authorization_server` inside the block to advertise it (see [Discovery](concepts/discovery.md)). Repeat the block to trust several issuers with distinct keys.
- The pre-1.0 top-level directives `publisher_jwt`, `subscriber_jwt`, `publisher_jwks_url` and `subscriber_jwks_url` still parse but map to a single implicit issuer usable only in compatibility mode; migrate them into an `issuer` block for modern mode.
- Keep redacting the legacy `authorization` and `access_token` query parameters from logs; old clients may still send them.
- `transport_url` (deprecated since 0.17) is removed; use `transport <name> { ... }`. The legacy non-Caddy server is removed.

### Compatibility mode

0.x behaviors are gated behind two build tags, honored only with `protocol_version_compatibility 8`:

- `deprecated_topic`: URI Template selectors in `topic=`, bare-string JWT matcher claims, alternate topics, the `/subscriptions/{topic}` routes.
- `deprecated_claim`: the legacy `mercure` claim (string and object forms), the `https://mercure.rocks/` namespaced claim, `mercure.payload`, the `authorization` query parameter, the `mercureAuthorization` cookie, and tokens without `typ: at+jwt` / `aud`.

Official binaries and Docker images ship with both tags, so you can run `protocol_version_compatibility 8` during the migration. A hub built without a tag rejects the corresponding 0.x behavior outright. Custom builds must pass the tags to `go build`.

### Go API changes

- `Update.Topics` becomes `Update.Topic` (a single topic).
- `canReceive` / `canDispatch` are replaced by the internal authorization-detail grant logic.
- `NewHub` requires a resource identifier (set `WithResourceIdentifier` or `WithPublicURL`) when JWT auth is enabled in modern mode.

---

## Historical changes (0.x)

The entries below describe earlier upgrades. They are kept for users migrating across multiple major versions.

### Mercure 0.21 upgrade notes

When Mercure is compiled manually or used as a Go library, deprecated features are no longer included by default.

To re-enable deprecated transports, pass the `deprecated_transports` build tag when compiling Mercure:

```console
# Mercure 0.21 Upgrade Notes
go build -tags deprecated_transport
```

To re-enable the legacy HTTP server, pass the `deprecated_server` build tag.

Official binaries and Docker images still include deprecated features.

### Mercure 0.17 upgrade notes

The `MERCURE_TRANSPORT_URL` environment variable and the `transport_url` directive were deprecated in favor of the `transport` directive.

Before:

```caddyfile
# Mercure 0.17 Upgrade Notes
transport_url bolt://mercure.db?cleanup_frequency=0.2
```

After:

```caddyfile
# Mercure 0.17 Upgrade Notes
transport bolt {
  path mercure.db
  cleanup_frequency 0.2
}
```

To configure the transport via an environment variable, append the directive to `MERCURE_EXTRA_DIRECTIVES`. Avoid putting credentials there; use `{env.MY_VAR}` placeholders in a custom Caddyfile instead.

### Mercure 0.16.2 upgrade notes

`Caddyfile.dev` was renamed to `dev.Caddyfile` to match Caddy best practices.

### Mercure 0.14.4 upgrade notes

This release moved to Caddy 2.6, which removed single-hyphen long-form flags. Use `--config` instead of `-config`.

### Mercure 0.14.3 upgrade notes

The Prometheus metric `mercure_subscribers` was renamed `mercure_subscribers_connected` for better interoperability with Datadog and others.

### Mercure 0.14.1 upgrade notes

The default development key changed from `!ChangeMe!` to `!ChangeThisMercureHubJWTSecretKey!` to satisfy the spec's 256-bit minimum.

### Mercure 0.14 upgrade notes

The `Last-Event-ID` query parameter was renamed `last_event_id`. Update your clients.

Publishing public updates in topics not listed in `mercure.publish` was removed; use `["*"]` to keep the old behavior.

A `protocol_version_compatibility 7` directive was added to ease the transition. It has since been removed in 1.0.

### Mercure 0.13 upgrade notes

The `DEBUG` environment variable was removed. Set `GLOBAL_OPTIONS=debug` instead.

### Mercure 0.11 upgrade notes

The hub became a Caddy module. Standalone binaries are now custom Caddy builds. The legacy server stayed available with a `legacy` build prefix until 1.0.

Before switching, [migrate your configuration](deployment/configuration.md).

### Mercure 0.10 upgrade notes

The protocol changed substantially. Highlights:

- Targets are gone, replaced by topic selectors. Mark updates `private` and check the `mercure.publish` / `mercure.subscribe` claims.
- Subscription JSON-LD: `"@type": "https://mercure.rocks/Subscription"` -> `"type": "Subscription"`.
- `dispatch_subscriptions` -> `subscriptions`.
- `subscriptions_include_ip` removed; use `mercure.payload`.
- IDs are now URNs (`urn:uuid:...`).
- `*` as a topic became reserved.

### Mercure 0.8 upgrade notes

- Hub URL changed from `/hub` to `/.well-known/mercure`.
- `HISTORY_CLEANUP_FREQUENCY`, `HISTORY_SIZE`, `DB_PATH` collapsed into `TRANSPORT_URL`.
- `ACME_HOSTS`, `CORS_ALLOWED_ORIGINS`, `PUBLISH_ALLOWED_ORIGINS` switched to space-separated values.
- The Go library's public API was rewritten.
