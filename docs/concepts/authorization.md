---
title: "Mercure authorization with OAuth 2.0 access tokens"
description: "Mint, present, and validate OAuth 2.0 JWT access tokens for Mercure publishers and subscribers with authorization_details, RFC 6750 errors, cookies, and JWKS."
---

# Authorization

The Mercure hub is an [OAuth 2.0](https://www.rfc-editor.org/rfc/rfc6749) protected resource. Clients present a **JWT access token** ([RFC 9068](https://www.rfc-editor.org/rfc/rfc9068)); the token's `authorization_details` claim ([RFC 9396](https://www.rfc-editor.org/rfc/rfc9396)) says which topics it may publish to and subscribe to. The hub validates every token; your application (or your authorization server) mints them.

> **Upgrading from 0.x?** The bespoke `mercure` claim is gone. Tokens are now standard OAuth 2.0 access tokens: a `typ: at+jwt` header, `iss` and `aud` claims, and an `authorization_details` array of Mercure entries. The legacy `mercure` claim is accepted only by a hub built with the `deprecated_claim` tag and running `protocol_version_compatibility 8`. See the [upgrade guide](../UPGRADE.md#10-from-0x).

## The access token

A Mercure access token is a JWT access token as defined by [RFC 9068](https://www.rfc-editor.org/rfc/rfc9068):

```jsonc
// header
{ "alg": "HS256", "typ": "at+jwt" }
```

```jsonc
// payload
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
        { "match": "https://example.com/users/42/notifications" },
        {
          "match": "https://example.com/books/:id",
          "match_type": "urlpattern",
        },
      ],
      "payload": { "user": "https://example.com/users/42" },
    },
  ],
}
```

The hub enforces, on every token:

- **`typ: at+jwt` header.** Tokens minted for other purposes (an OpenID Connect ID token, for example) are rejected.
- **`iss` claim.** It must exactly match one of the hub's trusted issuers: the `trusted_issuers` directive (your app's stable identifier when tokens are self-issued) or an entry of `authorization_servers` (see [Discovery](discovery.md)).
- **`aud` claim.** It must contain the hub's resource identifier (configured with `resource_identifier`, defaulting to `public_url`). `aud` may be a string or an array.
- **`exp` claim.** Required. The hub rejects expired tokens, including on the first request. `nbf` is enforced when present.
- **Signature** with the configured key (`publisher_jwt` / `subscriber_jwt`, or a JWKS; see below). The algorithm comes from hub configuration, never from the token, so `alg=none` and algorithm-confusion attacks are blocked.

[RFC 9068](https://www.rfc-editor.org/rfc/rfc9068) also requires issuers to populate `sub`, `client_id`, `iat`, and `jti`; include them so any RFC 9068 validator accepts your tokens. The hub uses `sub` to derive subscriber identifiers for [subscription events](active-subscriptions.md).

### Authorization details

Each entry in `authorization_details` with `"type": "https://mercure.rocks/authorization-detail"` grants a set of actions over a set of topic matchers:

- `actions`: a non-empty subset of `["publish", "subscribe"]`.
- `topics`: a non-empty array of [topic matcher](topics-and-matchers.md) objects `{ "match": "...", "match_type": "exact" | "urlpattern" }`. Bare strings are rejected; `match_type` is case-sensitive and defaults to `exact`. A `match` of `*` matches every topic.
- `payload` (optional, `subscribe` only): any JSON value, surfaced through [subscription events](active-subscriptions.md).

One invalid Mercure detail rejects the whole token (`401 invalid_token`); there is no partial acceptance. Entries with another `type` are ignored, so a single token can carry authorization details for several resources.

## Two ways to send the token

The hub reads the token from one of two places. Pick the one that matches your client:

1. **`Authorization: Bearer <token>` header (preferred).** Right for server-side code, mobile apps, command-line tools, and browser code using `fetch()`: anything that can set custom headers. In the browser, consume the SSE stream through the `fetch()` response body when you need a per-tab or per-connection token, or when the hub lives on another domain — cases a cookie can't cover. The `Bearer` scheme name is matched case-insensitively.
2. **`mercure_access_token` cookie (for `EventSource`).** Browsers can't attach headers to an `EventSource`; a cookie set with `HttpOnly`, `Secure`, and `SameSite` keeps the token out of JavaScript (no XSS exfiltration), out of URL bars and history, and rides along automatically. Set it at discovery time so it's already in place when the SSE connection opens.

There is no query-parameter mechanism: [RFC 9700](https://www.rfc-editor.org/rfc/rfc9700) forbids passing access tokens in URLs, where they leak into proxy logs, browser history, and `Referer` headers. When a request carries both a header and a cookie, the header wins and the cookie is ignored.

The hub never accepts tokens over plain HTTP. Whichever method you pick, **HTTPS is mandatory** for any non-anonymous request.

## Publishers

To publish, a token must carry an `authorization_details` entry whose `actions` include `publish` and whose `topics` match the update's topic.

```jsonc
// Publishers
{
  "authorization_details": [
    {
      "type": "https://mercure.rocks/authorization-detail",
      "actions": ["publish"],
      "topics": [
        {
          "match": "https://example.com/books/:id",
          "match_type": "urlpattern",
        },
        { "match": "https://example.com/announcements" },
      ],
    },
  ],
}
```

Behavior:

- No `publish` grant covering the topic -> the publication is rejected with `403 insufficient_scope`.
- An update has exactly one topic; the grant must cover that topic.
- `[{ "match": "*" }]` -> every topic is allowed.

`*` is the only "match anything" wildcard; you cannot get the same effect with a permissive URL Pattern.

## Subscribers

A subscriber's token is **only consulted for private updates**. Public updates flow to any subscriber whose `match*` query parameters hit, with or without a token.

For a private update, the hub checks that a `subscribe` grant covers the update's (single) topic. If it does, the update is delivered; if not, the subscriber never sees it.

```jsonc
// Subscribers
{
  "authorization_details": [
    {
      "type": "https://mercure.rocks/authorization-detail",
      "actions": ["subscribe"],
      "topics": [
        {
          "match": "https://example.com/users/42/:resource",
          "match_type": "urlpattern",
        },
        { "match": "https://example.com/announcements" },
      ],
    },
  ],
}
```

A `subscribe` grant of `[{ "match": "*" }]` receives every private update. No `subscribe` grant means no private updates.

### Anonymous subscribers

A hub with the `anonymous` directive set (development mode sets it for you) accepts subscribers without a token. Anonymous subscribers receive only updates that are **not** marked private; they have no grant to check against.

This is the right default for live feeds, public dashboards, and any case where the data isn't user-specific. For everything else, leave `anonymous` off.

## Per-user authorization on shared resources

A subscriber should receive updates only about the resources it owns. Because an update has exactly one topic and the hub authorizes against that single topic, you express this with a **scoped matcher** in the token, not with shared "capability" topics.

Publish each private update to its own per-user (or per-resource) topic:

```console
# Per-user authorization on shared resources
curl -X POST $HUB -H "Authorization: Bearer $JWT" \
  -d 'topic=https://example.com/users/42/messages/1' \
  -d 'private=on' \
  -d 'data=...'
```

Mint each subscriber a token whose `subscribe` grant covers only its own space:

```jsonc
// Per-user authorization on shared resources
{
  "authorization_details": [
    {
      "type": "https://mercure.rocks/authorization-detail",
      "actions": ["subscribe"],
      "topics": [
        {
          "match": "https://example.com/users/42/:resource",
          "match_type": "urlpattern",
        },
      ],
    },
  ],
}
```

User 42's token matches `https://example.com/users/42/messages/1`; user 99's token does not, so the hub never delivers it. The subscriber's `match*` query parameter can be as broad as `match_urlpattern=https://example.com/users/:id/messages/:mid`: the query selects what the client wants to receive, and the token decides what it is allowed to receive. The narrower of the two wins.

## Subscriber payloads

A `subscribe` detail can carry a `payload` (any JSON value). The hub attaches it to the [subscription event](active-subscriptions.md) and the [subscription API](active-subscriptions.md#subscription-api) record for every subscription that detail authorizes.

```jsonc
// Subscriber payloads
{
  "authorization_details": [
    {
      "type": "https://mercure.rocks/authorization-detail",
      "actions": ["subscribe"],
      "topics": [{ "match": "https://example.com/users/42" }],
      "payload": { "username": "alice", "ip": "10.0.0.1" },
    },
    {
      "type": "https://mercure.rocks/authorization-detail",
      "actions": ["subscribe"],
      "topics": [
        {
          "match": "https://example.com/books/:id",
          "match_type": "urlpattern",
        },
      ],
      "payload": { "username": "alice" },
    },
  ],
}
```

For each topic the subscriber asks for, the hub finds the first `subscribe` detail whose `topics` match it and attaches that detail's `payload`. Use payloads to ship per-subscriber metadata to other subscribers via subscription events: usernames, group memberships, IP address, role.

## RFC 6750 error responses

The hub answers authorization failures with standard [RFC 6750](https://www.rfc-editor.org/rfc/rfc6750) bearer-token errors:

| Situation                                                                                       | Status | Body / header                                                        |
| ----------------------------------------------------------------------------------------------- | ------ | -------------------------------------------------------------------- |
| No token on an operation that needs one                                                         | `401`  | bare `WWW-Authenticate: Bearer` with a `resource_metadata` parameter |
| Token presented but invalid (signature, `aud`, `exp`, `typ`, malformed `authorization_details`) | `401`  | `WWW-Authenticate: Bearer error="invalid_token"`                     |
| Valid token, but no grant for the action on the topic                                           | `403`  | `error="insufficient_scope"`                                         |
| Malformed request                                                                               | `400`  | `error="invalid_request"`                                            |

The `resource_metadata` parameter points clients at the hub's [protected resource metadata](discovery.md) so they can discover where to obtain a token. Error descriptions are deliberately terse: the hub never discloses _why_ a token failed (a valid signature over malformed claims still returns `invalid_token`).

## Cookies in detail

Set the cookie during discovery, when the user fetches the page or the API resource that links to the hub. By the time the browser opens the SSE connection, the cookie is already in place.

```http
# Cookies in detail
HTTP/1.1 200 OK
Set-Cookie: mercure_access_token=<JWT>; Domain=example.com; Path=/.well-known/mercure; Secure; HttpOnly; SameSite=Strict
Link: <https://hub.example.com/.well-known/mercure>; rel="mercure"
```

Required attributes:

- `Secure`: only sent over HTTPS.
- `HttpOnly`: not readable from JavaScript (XSS protection).
- `SameSite=Strict` or `Lax`: CSRF protection.
- `Path=/.well-known/mercure`: limits the cookie to the hub URL.

The default cookie name is `mercure_access_token`; override it with the `cookie_name` directive when several hubs share a domain. If the publisher and the hub run on different subdomains of the same registrable domain, set `Domain=example.com`. If they're on different domains, you can't use cookies; consume the stream with `fetch()` and an `Authorization` header instead.

`EventSource` does **not** send cookies on cross-origin requests by default. Pass `withCredentials: true` to opt in:

```javascript
// Cookies in detail
new EventSource(url, { withCredentials: true });
```

The hub must respond with the right CORS headers; a wildcard `cors_origins *` disables credentials, since the protocol forbids combining `Access-Control-Allow-Origin: *` with credentials. See [Configuration](../deployment/configuration.md#cors).

## Token expiration

The `exp` claim is required. The hub closes the subscriber's connection when the token expires; the browser auto-reconnects, and the now-expired token fails with `401 invalid_token`.

To handle expiry cleanly:

- Keep `exp` short enough to limit the blast radius of a leaked token (minutes to hours, not days).
- On the application side, refresh the token before it expires and update the cookie. The next reconnection picks up the new one.
- For long-lived sessions, run a small endpoint on your origin that mints a fresh hub token in exchange for the user's session, or front the hub with an OAuth 2.0 authorization server.

## Validating with JWKS

When an identity provider or authorization server (Keycloak, Cognito, Auth0) issues the tokens, point the hub at its JWKS endpoint instead of hardcoding a key:

```caddyfile
# Validating with JWKS
mercure {
  publisher_jwks_url https://idp.example.com/.well-known/jwks.json
  subscriber_jwks_url https://idp.example.com/.well-known/jwks.json
}
```

The hub fetches and caches the keys, rotates them when the provider does, and validates each token against the matching `kid`. See [Configuration](../deployment/configuration.md#jwt-validation-via-jwks).

## Verifying tokens with RSA and ECDSA keys

The default algorithm is HS256 (symmetric HMAC). For asymmetric verification (the hub holds only the public key), set the `*_JWT_ALG` environment variable or pass the algorithm as the second argument of the directive:

```caddyfile
# Verifying tokens with RSA and ECDSA keys
mercure {
  publisher_jwt {env.PUBLISHER_PUBLIC_KEY} RS256
  subscriber_jwt {env.SUBSCRIBER_PUBLIC_KEY} RS256
}
```

Asymmetric keys keep the signing key off the hub entirely, which is useful when the hub is operated by a different team than the publisher, or when an external authorization server mints the tokens.

## Common authorization errors

| Symptom                                    | Cause                                                                                                                    |
| ------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------ |
| `401 invalid_token` on subscribe           | Expired token, missing/wrong `iss` or `aud`, missing `typ: at+jwt`, malformed `authorization_details`, wrong signing key |
| `401` with a bare `Bearer` challenge       | No token presented on an operation that requires one                                                                     |
| `403 insufficient_scope` on publish        | No `publish` grant covers the topic                                                                                      |
| Subscriber never receives a private update | No `subscribe` grant covers the update's topic                                                                           |
| Browser doesn't send the cookie            | Missing `withCredentials: true`, wrong `Domain`/`Path`, or cross-origin without CORS credentials                         |

[Troubleshooting](../production/troubleshooting.md) covers each of these in more detail.
