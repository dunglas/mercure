# Upgrade

## 1.0

Topic matching now follows the topic matchers model of the revised specification:
two matcher types, Exact and [URL Pattern](https://urlpattern.spec.whatwg.org/).

### Query parameters

Subscribe parameters move from `topic`/`topicURLPattern` to the `match`
namespace, where the parameter name encodes the matcher type:

- `match` selects exact, case-sensitive comparison (the default type). Replace
  the 0.x `topic` parameter with `match`; its implicit
  [URI Template (RFC 6570)](https://tools.ietf.org/html/rfc6570) support is
  removed. `matchExact` is an explicit alias.
- `match<MatcherType>` selects a named matcher type: replace `topicURLPattern`
  with `matchURLPattern`, e.g. `matchURLPattern=https://example.com/books/:id`
  (note `:id`, not `{id}`).
- Parameter names are case-sensitive; any other name in the reserved `match`
  namespace is rejected with a `400 Bad Request`. The bare `match` mirrors the
  optional, `Exact`-defaulting `matchType` of authorization details.
- Relative patterns and topics are resolved against the hub URL: set it with
  the `public_url` directive (Caddyfile) or `WithPublicURL` (Go).

### Access tokens

The hub is now an OAuth 2.0 protected resource. Access tokens **must** be JWT
access tokens ([RFC 9068](https://www.rfc-editor.org/rfc/rfc9068)): the `typ`
header is `at+jwt` and the `aud` claim must contain the hub's resource
identifier. Set it with the `resource_identifier` directive (Caddyfile) or
`WithResourceIdentifier` (Go); it defaults to the public URL. A token-validating
hub started in modern mode without a resource identifier (nor a public URL)
fails to start.

The bespoke `mercure` claim is replaced by the standard `authorization_details`
claim ([RFC 9396](https://www.rfc-editor.org/rfc/rfc9396)):

```json
{
  "aud": "https://example.com/.well-known/mercure",
  "exp": 1735689600,
  "authorization_details": [
    { "type": "mercure", "actions": ["publish"], "topics": [{ "match": "*" }] },
    {
      "type": "mercure",
      "actions": ["subscribe"],
      "topics": [
        { "match": "https://example.com/books/:id", "matchType": "URLPattern" }
      ],
      "payload": { "foo": "bar" }
    }
  ]
}
```

`matchType` is case-sensitive and defaults to `Exact`. A `mercure` detail must
declare a non-empty `actions` array (a subset of `publish`/`subscribe`) and a
non-empty `topics` array; an invalid detail rejects the whole token with a
`400 Bad Request`. The optional per-detail `payload` is attached to the
subscriptions covered by a `subscribe` detail.

The access token is presented with an `Authorization: Bearer` header, an
`access_token` query parameter (the `authorization` parameter is removed), or
the authorization cookie.

### Authorization cookie

The default authorization cookie is renamed from `mercureAuthorization` to
`mercureAccessToken`, so its name reflects the OAuth 2.0 access token it carries
and matches the `access_token` query parameter. Browser subscribers relying on
the default name must set the new cookie. Hubs that configure a custom name with
the `cookie_name` directive (`WithCookieName` in Go) are unaffected.

The old `mercureAuthorization` name is accepted as a fallback only by hubs built
with the `deprecated_claim` build tag and running in compatibility mode (see
[Backward compatibility](#backward-compatibility)). A modern hub ignores it,
which makes it easy to tell which protocol version a client targets.

### Authorization errors

Authorization failures follow
[RFC 6750](https://www.rfc-editor.org/rfc/rfc6750): a `401` with a bare
`WWW-Authenticate: Bearer` challenge (carrying a `resource_metadata` parameter)
when no token is presented, `401 invalid_token` when a token fails validation,
`403 insufficient_scope` when a valid token lacks the requested action on the
topic, and `400 invalid_request` for malformed authorization requests.

### Discovery

The hub serves OAuth 2.0 Protected Resource Metadata
([RFC 9728](https://www.rfc-editor.org/rfc/rfc9728)) at
`/.well-known/oauth-protected-resource/.well-known/mercure`. Advertise
authorization servers with the `authorization_servers` directive
(`WithAuthorizationServers` in Go).

### Updates

An update has exactly one topic: publish requests with several `topic` fields
are rejected with a `400 Bad Request`, and `Update.Topics` is replaced by
`Update.Topic` in the Go API. Publish one update per topic instead of using
alternate topics.

### Subscription API

Subscription URLs move from `/subscriptions/{topic}[/{subscriber}]` to
`/subscriptions/{matchType}/{match}[/{subscriber}]`, and the JSON-LD documents
expose `match` and `matchType` fields instead of `topic`.

### Backward compatibility

The 0.x topic behaviors (URI Template selectors in `topic`, alternate topics
and the previous subscription routes) require both:

1. a hub binary built with the `deprecated_topic` build tag (official binaries
   and Docker images include it), and
2. the `protocol_version_compatibility 8` directive
   (`WithProtocolVersionCompatibility(8)` in Go).

The 0.x authorization behaviors (the `mercure` JWT claim in string and object
forms, the `https://mercure.rocks/` namespaced claim, `mercure.payload`, the
`authorization` query parameter, the `mercureAuthorization` cookie name, and
tokens without `typ: at+jwt`/`aud`) require the `deprecated_claim` build tag
(also included in official builds) and the same `protocol_version_compatibility 8`
directive.

Note: bolt databases written by 0.x hubs stay readable, but replaying history
recorded with alternate topics only matches them in `deprecated_topic` builds.

## 0.21

When Mercure is compiled manually or used as a Go library, deprecated features are no longer included by default.

To re-enable deprecated transports, pass the `deprecated_transports` build tag when compiling Mercure:

```console
go build -tags deprecated_transport
```

To re-enable the legacy HTTP server, pass the `deprecated_server` build tag.

Official binaries and Docker images still include deprecated features.

## 0.17

The `MERCURE_TRANSPORT_URL` environment variable and the `transport_url` directive have been deprecated.
Use the new `transport` directive instead.

The `MERCURE_TRANSPORT_URL` environment variable has been removed from the default `Caddyfile`s,
but a backward compatibility layer is provided.

If both the `transport` and the deprecated `transport_url` are not explicitly set
and the `MERCURE_TRANSPORT_URL` environment variable is set, the `transport_url` will be automatically populated.
To disable this behavior, unset `MERCURE_TRANSPORT_URL` or set it to an empty string.

Before:

```caddyfile
transport_url bolt://mercure.db?cleanup_frequency=0.2
```

After:

```caddyfile
transport bolt {
  path mercure.db
  cleanup_frequency 0.2
}
```

To configure the transport using an environment variable, append the `transport` directive to the `MERCURE_EXTRA_DIRECTIVES` environment variable:

```console
MERCURE_EXTRA_DIRECTIVES="transport bolt {
  path mercure.db
}"
```

To prevent security issues, be sure to not pass credentials such as API tokens or password in `MERCURE_EXTRA_DIRECTIVES` (ex: when using transports [provided by the paid version](hub/cluster.md) such as Redis).

To pass credentials security, create a custom `Caddyfile` and use the `{env.MY_ENV_VAR}` syntax, which is interpreted at runtime.

## 0.16.2

The `Caddyfile.dev` file has been renamed `dev.Caddyfile` to match new Caddy best practices
and prevent "ambiguous adapter" issues.

## 0.14.4

This release is built on top of [Caddy 2.6](https://github.com/caddyserver/caddy/releases/tag/v2.6.0).
Caddy 2.6 removed support for single-hyphen long-form flags (such as `-config`), use the double-hyphen syntax instead (`--config`).

## 0.14.3

The `mercure_subscribers` field of the Prometheus endpoint has been renamed `mercure_subscribers_connected` for better interoperability (including with Datadog).

## 0.14.1

The default dev key changed from `!ChangeMe!` to `!ChangeThisMercureHubJWTSecretKey!` to respect the specification (the key must be longer than 256 bits).

## 0.14

The query parameter allowing you to fetch past events has been renamed `lastEventID`: in your clients, replace all occurrences of the `Last-Event-ID` query parameter with `lastEventID`.

Publishing public updates in topics not explicitly listed in the `mercure.publish` JWT claim isn't supported anymore.
To let your publishers publish (public and private updates) in all topics, use the special `*` topic selector:

```patch
 {
   "mercure": {
-    "publish": []
+    "publish": ["*"]
 }
```

Backward compatibility with the old version of the protocol (version 7) can be enabled by setting the `protocol_version_compatibility` directive to `7` in your `Caddyfile`.

## 0.13

The `DEBUG` environment variable has gone. Set the `GLOBAL_OPTIONS` environment variable to `debug` instead.

## 0.11

The Mercure.rocks Hub is now available as a module for the [Caddy web server](https://caddyserver.com/).
It is also easier to use as [a standalone Go library](https://pkg.go.dev/github.com/dunglas/mercure).
We still provide standalone binaries, but it's now a custom build of Caddy including the Mercure module.

Builds of the legacy server are also available to ease the transition, but starting with version 0.12 only the Caddy-based builds will be provided (they have the `legacy` prefix).

Relying on Caddy allows to use the Mercure.rocks Hub as a [reverse proxy](https://caddyserver.com/docs/quick-starts/reverse-proxy) for your site or API that also adds the Mercure well-known URL (`/.well-known/mercure`). Thanks to this new feature, the well-known URL can be on the same domain as your site or API, so you don't need to deal with [CORS](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS).

All features provided by Caddy are also supported by this custom build: [HTTP/3 and h2c support](https://caddyserver.com/docs/json/apps/http/servers/#experimental_http3), [compression](https://caddyserver.com/docs/caddyfile/directives/encode), [Prometheus metrics](https://caddyserver.com/docs/metrics) (with additional Mercure-specific metrics), profiler (`/debug/pprof/`)...

Before switching to the Caddy build, be sure to [migrate your configuration](hub/config.md).

## 0.10

This version is in sync with the latest version of the specification, which changed a lot. Upgrading to 0.10 **requires to change your code**. Carefully read this guide before upgrading the hub.

- Private updates are now handled differently. _Targets_ don't exist anymore. They have been superseded by the concept of _topic selectors_.
  To send a private update, the publisher must now set the new `private` field to `on` when sending the `POST` request. The topics of the update must also match at least one selector (a URI Template, a raw string or `*` to match all topics) provided in the `mercure.publish` claim of the JWT.
  To receive a private update, at least one topic of this update must match at least one selector provided in the `mercure.subscribe` claim of the JWT.
- The structure of the JSON-LD document included in subscription events changed. Especially, `"@type": "https://mercure.rocks/Subscription"` is now `"type": "Subscription"` and `"@id": "/.well-known/mercure/subscriptions/foo/bar"` is now `"id": "/.well-known/mercure/subscriptions/foo/bar"`.
- The `dispatch_subscriptions` config option has been renamed `subscriptions`.
- The `subscriptions_include_ip` config option doesn't exist anymore. To include the subscriber IP (or any other value) in subscription events, use the new `mercure.payload` property of the JWT.
- All IDs generated by the hub (updates ID, subscriptions IDs...) are now URN following the template `urn:uuid:{the-uuid}` (it was `{the-uuid}` before). You may need to update your code if you deal with these IDs.
- The topic `*` is now reserved and allows to subscribe to all topics.

## 0.8

- According to the new version of the spec, the URL of the Hub has changed from `/hub` to `/.well-known/mercure`.
- `HISTORY_CLEANUP_FREQUENCY`, `HISTORY_SIZE` and `DB_PATH` environment variables have been replaced by the new `TRANSPORT_URL` environment variable
- Lists in `ACME_HOSTS`, `CORS_ALLOWED_ORIGINS`, `PUBLISH_ALLOWED_ORIGINS` must now be space separated
- The public API of the Go library has been totally revamped
