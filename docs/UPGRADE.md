---
title: "Mercure 1.0 Upgrade Guide"
description: "Step-by-step migration from Mercure 0.x to 1.0: typed matcher query parameters, object-form JWT claims, and the new subscription event topic format."
---

# Upgrade Guide

## 1.0 (from 0.x)

The 1.0 release is the first version aligned with the IETF specification's typed-matcher model. It is a **breaking change** for both subscribers and JWT issuers. Publishers are unaffected.

If you only run the hub and don't author subscribers or mint JWTs yourself, the upgrade is a config change. If you do, plan a synchronized cutover of the hub and the clients that talk to it.

### What Changed at a Glance

| Area | 0.x | 1.0 |
| --- | --- | --- |
| Subscribe query parameter | `topic=<pattern>` (URI Template or string) | `match=<exact>`, `matchURLPattern=<pattern>`, `matchRegexp=<pattern>`, ... |
| Default templating language | URI Templates ([RFC 6570](https://www.rfc-editor.org/rfc/rfc6570)) | URL Patterns ([WHATWG](https://urlpattern.spec.whatwg.org)) |
| `mercure.subscribe` / `mercure.publish` claim | Array of strings | Array of objects `{match, matchType, payload}` |
| Wildcard | `"*"` (string) | `{"match": "*"}` (object) |
| Subscription event topic | `/.well-known/mercure/subscriptions/{topic}/{subscriber}` | `/.well-known/mercure/subscriptions/{matchType}/{match}/{subscriber}` |
| Subscription JSON-LD | `topic` | `match` + `matchType` |
| Backward-compat mode | `protocol_version_compatibility 7` | Removed |

### Migrate Your Subscribers

The single change is the query parameter name.

**Before (0.x):**

```javascript
// Migrate your subscribers
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append("topic", "https://example.com/books/1");
url.searchParams.append("topic", "https://example.com/books/{id}");
new EventSource(url);
```

**After (1.0):**

```javascript
// Migrate your subscribers
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append("match", "https://example.com/books/1");
url.searchParams.append("matchURLPattern", "https://example.com/books/:id");
new EventSource(url);
```

Two things to notice:

1. The exact-match parameter is now `match` (alias: `matchExact`). The hub treats query parameter names case-insensitively for any name starting with `match`.
2. The templated parameter is `matchURLPattern`, and the syntax is [URL Patterns](https://urlpattern.spec.whatwg.org) (`/:id`, not `/{id}`).

If your existing patterns are URI Templates and you'd rather not rewrite them, the hub still supports them via `matchURITemplate`. New code should use URL Patterns — they're better-defined for URLs and the only matcher type natively supported by browsers.

### Migrate Your JWTs

The `mercure.publish` and `mercure.subscribe` claims must now contain **objects**, not bare strings. The hub rejects bare strings with a `401 Unauthorized` and refuses to mint a session.

**Before (0.x):**

```jsonc
// Migrate your JWTs
{
  "mercure": {
    "publish": ["*"],
    "subscribe": [
      "https://example.com/users/42",
      "https://example.com/books/{id}"
    ]
  }
}
```

**After (1.0):**

```jsonc
// Migrate your JWTs
{
  "mercure": {
    "publish": [
      { "match": "*" }
    ],
    "subscribe": [
      { "match": "https://example.com/users/42" },
      { "match": "https://example.com/books/:id", "matchType": "URLPattern" }
    ]
  }
}
```

Rules:

- `matchType` defaults to `"Exact"` if omitted, so any plain URL or string can stay as `{ "match": "<value>" }`.
- The reserved value `{ "match": "*" }` matches every topic. It's the equivalent of the old `"*"` string.
- `matchType` is case-insensitive: `"URLPattern"`, `"urlpattern"`, and `"UrlPattern"` are equivalent.
- The `payload` field is per-claim-entry, with [explicit fallback rules](concepts/authorization.md#mercure-subscriber-payloads) when several entries match.

### Migrate the Subscription API and Events

The route pattern and the JSON-LD shape changed.

| Before | After |
| --- | --- |
| `/.well-known/mercure/subscriptions/<topic>/<subscriber>` | `/.well-known/mercure/subscriptions/<matchType>/<match>/<subscriber>` |
| `"topic": "https://..."` in the JSON-LD | `"match": "https://..."` and `"matchType": "URLPattern"` |

`<matchType>`, `<match>`, and `<subscriber>` must be percent-encoded. See [Active subscriptions](concepts/active-subscriptions.md) for the new layout.

### Compatibility Mode Is Gone

In 0.14, the `protocol_version_compatibility 7` directive let the hub speak the old protocol while you migrated. 1.0 removes it. The reasoning is that the JWT claim form changed from string to object, and silently re-interpreting old tokens under the new rules would change their meaning — that's a security risk, not a convenience. Mint new tokens.

### Mercure 1.0 Find-and-Replace Checklist

Search your codebase for these patterns:

- `?topic=` and `&topic=` in subscriber URLs → `match=` (or `matchURLPattern=` if templated)
- `searchParams.append("topic"` / `appendParam("topic"` → `"match"`
- URI Template syntax in subscribe URLs (`{id}`, `{+host}`) → URL Pattern syntax (`:id`, `:host`)
- `"publish": ["*"]` in JWT issuer code → `"publish": [{"match": "*"}]`
- `"subscribe": ["..."]` in JWT issuer code → `"subscribe": [{"match": "..."}]`
- Hardcoded `subscriptions/{topic}/{subscriber}` paths → add the `{matchType}` segment

Once your services emit and parse the new shapes, switch the hub to 1.0.

### Mercure Hub Configuration Changes in 1.0

Two directives that no longer exist:

- `protocol_version_compatibility` — removed.
- `transport_url` — removed (deprecated since 0.17). Use `transport <name> { ... }`.

The legacy non-Caddy server (deprecated since 0.11) is also removed. If you're still on it, see [Installation](getting-started/installation.md) for the current builds.

---

## Historical Changes (0.x)

The entries below describe earlier upgrades. They are kept for users migrating across multiple major versions.

### Mercure 0.21 Upgrade Notes

When Mercure is compiled manually or used as a Go library, deprecated features are no longer included by default.

To re-enable deprecated transports, pass the `deprecated_transports` build tag when compiling Mercure:

```console
# Mercure 0.21 Upgrade Notes
go build -tags deprecated_transport
```

To re-enable the legacy HTTP server, pass the `deprecated_server` build tag.

Official binaries and Docker images still include deprecated features.

### Mercure 0.17 Upgrade Notes

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

### Mercure 0.16.2 Upgrade Notes

`Caddyfile.dev` was renamed to `dev.Caddyfile` to match Caddy best practices.

### Mercure 0.14.4 Upgrade Notes

This release moved to Caddy 2.6, which removed single-hyphen long-form flags. Use `--config` instead of `-config`.

### Mercure 0.14.3 Upgrade Notes

The Prometheus metric `mercure_subscribers` was renamed `mercure_subscribers_connected` for better interoperability with Datadog and others.

### Mercure 0.14.1 Upgrade Notes

The default development key changed from `!ChangeMe!` to `!ChangeThisMercureHubJWTSecretKey!` to satisfy the spec's 256-bit minimum.

### Mercure 0.14 Upgrade Notes

The `Last-Event-ID` query parameter was renamed `lastEventID`. Update your clients.

Publishing public updates in topics not listed in `mercure.publish` was removed; use `["*"]` to keep the old behavior.

A `protocol_version_compatibility 7` directive was added to ease the transition. It has since been removed in 1.0.

### Mercure 0.13 Upgrade Notes

The `DEBUG` environment variable was removed. Set `GLOBAL_OPTIONS=debug` instead.

### Mercure 0.11 Upgrade Notes

The hub became a Caddy module. Standalone binaries are now custom Caddy builds. The legacy server stayed available with a `legacy` build prefix until 1.0.

Before switching, [migrate your configuration](deployment/configuration.md).

### Mercure 0.10 Upgrade Notes

The protocol changed substantially. Highlights:

- Targets are gone, replaced by topic selectors. Mark updates `private` and check the `mercure.publish` / `mercure.subscribe` claims.
- Subscription JSON-LD: `"@type": "https://mercure.rocks/Subscription"` → `"type": "Subscription"`.
- `dispatch_subscriptions` → `subscriptions`.
- `subscriptions_include_ip` removed; use `mercure.payload`.
- IDs are now URNs (`urn:uuid:...`).
- `*` as a topic became reserved.

### Mercure 0.8 Upgrade Notes

- Hub URL changed from `/hub` to `/.well-known/mercure`.
- `HISTORY_CLEANUP_FREQUENCY`, `HISTORY_SIZE`, `DB_PATH` collapsed into `TRANSPORT_URL`.
- `ACME_HOSTS`, `CORS_ALLOWED_ORIGINS`, `PUBLISH_ALLOWED_ORIGINS` switched to space-separated values.
- The Go library's public API was rewritten.
