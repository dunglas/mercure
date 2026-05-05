# Troubleshooting

The greatest hits, in roughly the order you're likely to hit them.

## 401 Unauthorized

Causes, in priority order:

1. **No token presented.** Check that the request carries an `Authorization` header, an `authorization` query parameter, or the `mercureAuthorization` cookie. For browsers, `EventSource(url, { withCredentials: true })` is required for cross-origin requests.
2. **Bare-string claim entries (0.x format).** In 1.0, `mercure.subscribe` and `mercure.publish` claims must contain **objects** (`{match, matchType, payload}`), not strings. The hub rejects bare strings with `401`. See [Authorization](../concepts/authorization.md) and the [upgrade guide](../UPGRADE.md#10-from-0x).
3. **Wrong key or algorithm.** The hub verifies with the configured key + algorithm. If your token is signed with HS256 and the hub is set to RS256, it fails. Check `MERCURE_*_JWT_KEY` and `MERCURE_*_JWT_ALG`.
4. **Expired `exp`.** Browsers auto-reconnect with the same token after disconnect; once it expires, every reconnect fails. Mint a fresh token on the application side and update the cookie.
5. **Special characters in the key.** Shell escaping, YAML parsing, and Kubernetes secret base64-encoding all bite. Verify the key as the hub sees it (`docker exec` and `printenv`).
6. **Anonymous mode disabled.** Without `anonymous` in the Caddyfile, subscribers without a token are rejected. Add a token or enable `anonymous` for public topics.

The hub logs the exact reason on `stderr`. Read the logs.

## 403 Forbidden on publish

The publisher's `mercure.publish` claim doesn't cover one or more of the topics in the publication.

```json
{
  "mercure": {
    "publish": [
      { "match": "https://example.com/books/:id", "matchType": "URLPattern" }
    ]
  }
}
```

- A publish to `https://example.com/books/42` works.
- A publish to `https://example.com/users/42` is rejected.
- A publish with two `topic` fields, where only one matches, is rejected entirely (the hub does **not** publish a partial subset).

Use `[{ "match": "*" }]` to allow every topic. Note the **object** form — `["*"]` (string) is not accepted.

## Subscriber never receives a private update

For a `private=on` update, the hub checks the subscriber's `mercure.subscribe` against **at least one** of the update's topics (canonical or alternate). If none match, the hub drops the update silently — no error, no log entry on the subscriber side.

Common shapes of this bug:

- The publish includes only the canonical topic and the subscriber's claim covers an alternate that's never set.
- The claim uses `Exact` (the default) but the topic is templated.
- The claim's URL Pattern is more restrictive than the subscriber's `match*` query parameter — the subscriber asks for `:id` but is only authorized for `/books/:id`.

The fix is usually to widen the claim, narrow the subscription, or use the [per-user authorization pattern](../concepts/authorization.md#per-user-authorization-on-shared-topics).

## CORS

Symptoms in the browser console:

- Chrome: `Refused to connect to 'https://hub.example.com/.well-known/mercure?match=...' because it violates the following Content Security Policy directive`
- Firefox: `Cross-Origin Request Blocked: ... CORS header 'Access-Control-Allow-Origin' missing`

Set the allowed origins in the Caddyfile:

```caddyfile
mercure {
  cors_origins https://app.example.com https://admin.example.com
}
```

Don't forget the `https://` prefix.

For credentialed requests (cookie or `Authorization`), `cors_origins *` does **not** work — browsers reject wildcard origins on credentialed requests. List the explicit origins.

If the hub is fully anonymous (no JWT, no cookie), `*` is fine, but understand the security implications.

For production, the cleanest fix is to host the hub on the same registrable domain as your app and avoid CORS entirely. See [Reverse proxies](../deployment/reverse-proxy.md#cors-via-reverse-proxy).

## URL patterns aren't matching

Test patterns in the browser console:

```javascript
new URLPattern("https://example.com/books/:id").test("https://example.com/books/42")
// → true
```

Common surprises:

- A trailing slash matters. `/books/:id` matches `/books/42` but not `/books/42/`.
- Patterns are matched against the full URL by default. Use a relative pattern (`/books/:id`) to match against just the path, with the hub URL as base.
- `:id` matches any non-`/` segment. Use `:rest*` for "any tail" matches.

For URI Templates in 0.x-compatible mode, the [URI Template tester](https://uri-template-tester.mercure.rocks/) is still online.

## Connection drops after a few minutes

If your subscribers reconnect like clockwork every 30, 60, or 120 seconds, an intermediate proxy is closing idle connections. Common culprits:

- NGINX with default `proxy_read_timeout 60s`. Raise to `24h`.
- Cloudflare Free / Pro plans have a 100s streaming proxy timeout.
- AWS ALB default idle timeout is 60s.
- Corporate proxies often kill long-lived connections at 5 or 30 minutes.

The hub sends a comment heartbeat every `heartbeat` seconds (default 40). If your proxy times out at 30s, lower `heartbeat` to e.g. `25s`.

## Disconnection with inability to reconnect after some time

If your JWT has an `exp` claim, the hub closes the connection at that time. The browser auto-reconnects with the same (now expired) token, fails with `401`, and gives up.

Two fixes:

- **Refresh the token before it expires.** Have your origin mint a fresh token; update the cookie. Next reconnect picks it up.
- **Don't set `exp`.** Open-ended connections; use only when the threat model genuinely accepts indefinite tokens.

In practice, refreshing is the right answer for almost all cases.

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

## "address already in use"

Port 80 or 443 is taken by another service (Apache, NGINX, sometimes Skype). Either stop it, or move the hub to a free port:

```console
SERVER_NAME=:3000 ./mercure run
```

Note: Let's Encrypt's HTTP-01 challenge needs port 80 or 443 to be reachable. If you move the hub off those, either disable Let's Encrypt or use the DNS-01 challenge.

## "too many open files"

The hub hit the OS file descriptor limit. Each subscriber takes one fd.

```console
ulimit -n 100000
```

For systemd services, set `LimitNOFILE=100000` in the unit file. For Docker, use `ulimits` in the compose file. See [Load testing](load-testing.md#file-descriptor-limits) for full details.

## Hub responds 405 Method Not Allowed

Expected. The hub only accepts `GET` (subscribe) and `POST` (publish) on `/.well-known/mercure`. `405` means the hub is up and responding; you sent the wrong method.

If you didn't send anything (no client, just `curl`), `405` is your readiness check.

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

## Self-Hosted: license errors

If you're running [Self-Hosted Mercure](https://mercure.rocks/pricing) and see license errors:

- Check `MERCURE_LICENSE` is set and the value isn't truncated (long keys are easy to truncate when copy-pasting).
- The check runs in-process; no callback to a license server. License errors are about the value of the env var, not network reachability.
- Connection cap exceeded → `429 Too Many Requests` to publishers, refusal of new subscribers. Upgrade your tier or shed connections.

Email [contact@mercure.rocks](mailto:contact@mercure.rocks) with your hub ID for license issues.

## When in doubt

- Read the hub's `stderr` logs.
- Capture a `goroutine?debug=2` dump (see [Debugging](debugging.md)).
- Compare your JWT payload against [Authorization](../concepts/authorization.md). Most 401/403 issues are JWT-shaped.
- Ask in [GitHub Discussions](https://github.com/dunglas/mercure/discussions) with a minimal repro.

> **Pro tip.** [Self-Hosted Mercure](https://mercure.rocks/pricing) tiers include direct email support from the maintainers, with priority next-day on Business and full SLAs on Corporate and Elite. If your hub is critical to your business, that's the simplest insurance.
