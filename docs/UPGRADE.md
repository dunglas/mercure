# Upgrade

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
