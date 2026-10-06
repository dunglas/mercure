---
title: "Mercure troubleshooting: 401, 403, CORS, and reconnect issues"
description: "Diagnose 401 / 403 errors, CORS failures, dropped SSE connections, and other common Mercure.rocks Hub issues with concrete fixes."
---

# Mercure troubleshooting

Find the symptom below, then check the request, token, and hub configuration.

## 401 unauthorized

The hub returns `401` either with a bare `WWW-Authenticate: Bearer` challenge (no token) or `error="invalid_token"` (a token that failed validation). Causes, in priority order:

1. **No token presented.** Check that the request carries an `Authorization: Bearer` header or the `__Secure-mercure_access_token` cookie (the `access_token` query parameter is not accepted). For browsers, `EventSource(url, { withCredentials: true })` is required for cross-origin requests.
2. **Missing `typ: at+jwt` header, wrong `iss`, or wrong `aud`.** Access tokens must use the `at+jwt` header type, carry an `iss` matching one of the hub's configured issuers (an `issuer` block), and an `aud` matching its `resource_identifier`. A plain `JWT` token, or one minted for a different issuer or audience, fails. See [Authorization](../concepts/authorization.md) and the [upgrade guide](../UPGRADE.md#10-from-0x).
3. **Malformed `authorization_details`.** Each Mercure authorization detail needs a non-empty `actions` array and a non-empty `topics` array of `{ match, match_type? }` objects. One bad detail rejects the whole token.
4. **Wrong key or algorithm.** The hub verifies with the configured key + algorithm. If your token is signed with HS256 and the hub is set to RS256, it fails. Check `MERCURE_*_JWT_KEY` and `MERCURE_*_JWT_ALG`.
5. **Expired `exp`.** `exp` is required. Browsers auto-reconnect with the same token after disconnect; once it expires, every reconnect fails. Mint a fresh token on the application side and update the cookie.
6. **Special characters in the key.** Shell escaping, YAML parsing, and Kubernetes secret base64-encoding all bite. Check quoting and compare keys without printing secrets into logs.
7. **Anonymous mode disabled.** Without `anonymous` in the Caddyfile, subscribers without a token are rejected. Add a token or enable `anonymous` for public topics.

The hub logs the exact reason on `stderr`. Read the logs.

## 403 insufficient_scope on publish

The token is valid but no `authorization_details` entry grants `publish` on the publication's topic.

```json
{
  "authorization_details": [
    {
      "type": "https://mercure.rocks/authorization-detail",
      "actions": ["publish"],
      "topics": [
        {
          "match": "https://example.com/books/:id",
          "match_type": "urlpattern"
        }
      ]
    }
  ]
}
```

- A publish to `https://example.com/books/42` works.
- A publish to `https://example.com/users/42` is rejected with `403 insufficient_scope`.

Use `"topics": [{ "match": "*" }]` to allow every topic.

## Subscriber never receives a private update

For a `private=on` update, the subscriber needs both a matching subscription and a `subscribe` grant covering at least one of the update's topics. Unauthorized updates are omitted from the stream without an error.

Common causes:

- The token's matcher uses `exact` (the default) but the topic needs a `urlpattern`.
- The token's URL Pattern is more restrictive than the subscriber's `match*` query parameter: the subscriber asks for `:id` but is only authorized for `/books/:id`.
- The subscriber forgot a token entirely (anonymous subscribers receive only public updates).

Check the intended access policy before changing a grant. See [Per-user authorization](../concepts/authorization.md#per-user-authorization-on-shared-resources).

## CORS

Check the browser console for CORS errors. A Content Security Policy error is separate: allow the hub origin in the page's `connect-src` directive. Typical CORS errors include:

- Chrome: a message stating that the request was blocked by CORS policy.
- Firefox: `Cross-Origin Request Blocked: ... CORS header 'Access-Control-Allow-Origin' missing`

Set the allowed origins in the Caddyfile:

```caddyfile
mercure {
  cors_origins https://app.example.com https://admin.example.com
}
```

Don't forget the `https://` prefix.

For cookie requests using `withCredentials: true`, `cors_origins *` does not work. List the application's exact origin. A bearer header sent with `fetch` also requires a successful preflight.

For requests without cookies, `cors_origins *` can allow access from any origin. Use it only when that is intentional.

To avoid CORS, serve the hub through a proxy on the **same origin** as the application. Sharing a registrable domain is not enough; subdomains are different origins. See [Reverse proxies](../deployment/reverse-proxy.md#cors-via-reverse-proxy).

## URL patterns aren't matching

Test patterns in the browser console:

```javascript
new URLPattern("https://example.com/books/:id").test(
  "https://example.com/books/42",
);
// -> true
```

Common surprises:

- A trailing slash matters. `/books/:id` matches `/books/42` but not `/books/42/`.
- Relative patterns (`/books/:id`) resolve against the hub URL; they also constrain the origin and do not match that path on every host.
- `:id` matches any non-`/` segment. Use `:rest*` for "any tail" matches.

For URI Templates in 0.x-compatible mode, the [URI Template tester](https://uri-template-tester.mercure.rocks/) is still online.

## Connection drops after a few minutes

For regular disconnects, check `write_timeout`, token expiry, and proxy idle timeouts. The default hub deadline is randomized between 480 and 600 seconds. Other common causes: Common culprits:

- NGINX with default `proxy_read_timeout 60s`. Raise to `24h`.
- Cloudflare applies product-specific [connection limits](https://developers.cloudflare.com/fundamentals/reference/connection-limits/).
- AWS ALB default idle timeout is 60s.
- Corporate proxies often kill long-lived connections at 5 or 30 minutes.

The hub sends a comment heartbeat every `heartbeat` seconds (default 40). If your proxy times out at 30s, lower `heartbeat` to e.g. `25s`.

## Disconnection with inability to reconnect after some time

The required `exp` claim bounds subscription lifetime. After an expired token produces a `401`, a native `EventSource` may stop reconnecting. Refresh the token and create a new `EventSource` if the previous one has closed.

Two fixes:

- **Refresh the token before it expires.** Have your origin mint a fresh token; update the cookie. Next reconnect picks it up.
- **Use a longer `exp` if you must.** RFC 9068 access tokens require `exp`, so it can't be omitted; widen the window only when the application accepts the risk of long-lived tokens.

Refresh tokens before they expire.

## macOS: "cannot be opened because the developer cannot be verified"

The binary is quarantined on first run. Strip the attribute once:

```console
xattr -d com.apple.quarantine ./mercure
```

Then start as usual:

```console
./mercure run
```

You only need to do this once per binary.

## "Address already in use"

Port 80 or 443 is taken by another service (Apache, NGINX, sometimes Skype). Either stop it, or move the hub to a free port:

```console
SERVER_NAME=:3000 ./mercure run
```

`SERVER_NAME=:3000` serves plain HTTP. For public HTTPS, HTTP-01 validation needs port 80 and TLS-ALPN-01 needs port 443. DNS-01 is an alternative when those ports are unavailable.

## "Too many open files"

The process reached its file-descriptor limit. Each TCP connection consumes a descriptor; HTTP/2 streams may share one.

```console
ulimit -n 100000
```

For systemd services, set `LimitNOFILE=100000` in the unit file. For Docker, use `ulimits` in the compose file. See [Load testing](load-testing.md#file-descriptor-limits-for-the-mercure-hub) for full details.

## Hub responds 405 method not allowed

A `405` means the route does not support the requested method. This hub accepts `GET`, `HEAD`, and `QUERY` for subscriptions, and `POST` for publications. A `GET` without matchers normally returns `400`, or `401` if authentication is required.

Use [transport readiness](health-monitoring.md#mercure-hub-health-endpoints) for health checks. An error response from the subscription endpoint is not a readiness check.

## Updates arrive in batches every few seconds

Reverse proxy is buffering. Set `proxy_buffering off` (NGINX) or the equivalent on your proxy. See [Reverse proxies](../deployment/reverse-proxy.md).

## Subscription events not firing

Check that `subscriptions` is in the Caddyfile:

```caddyfile
mercure {
  subscriptions
  # ...
}
```

It's off by default. Without it, the hub doesn't publish subscription events and the subscription API returns `404`.

## Self-hosted: license errors

If you're running [Self-Hosted Mercure](https://mercure.rocks/pricing) and see license errors:

- Check `MERCURE_LICENSE` is set and the value isn't truncated (long keys are easy to truncate when copypasting).
- The check runs in-process; no callback to a license server. License errors are about the value of the env var, not network reachability.
- Connection cap exceeded: `429 Too Many Requests` to publishers, refusal of new subscribers. Upgrade your tier or shed connections.

Email [contact@mercure.rocks](mailto:contact@mercure.rocks) with your hub ID for license issues.

## When in doubt: Mercure hub diagnostic steps

- Read the hub's `stderr` logs.
- Capture a `goroutine?debug=2` dump (see [Debugging](debugging.md)).
- Compare your JWT payload against [Authorization](../concepts/authorization.md). Check the header, claims, signature, and topic grants.
- Ask in [GitHub Discussions](https://github.com/dunglas/mercure/discussions) with a minimal repro.
