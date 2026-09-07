---
title: "Mercure 1.0 with Symfony and API Platform"
description: "Configure symfony/mercure-bundle 0.5 to speak the Mercure 1.0 protocol: protocol_version, the RFC 9068 claims the bundle requires, JWKS signing keys, cookie names, and the FrankenPHP caveat."
---

# Symfony and API Platform

Symfony publishes to Mercure through [`symfony/mercure`](https://github.com/symfony/mercure) and its bundle, [`symfony/mercure-bundle`](https://github.com/symfony/mercure-bundle). API Platform builds on the same bundle. Both support the 1.0 protocol from `symfony/mercure-bundle` 0.5.

Nothing in your publishing code changes: you still inject `HubInterface` and publish an `Update`. What changes is the shape of the token the bundle mints, and that is one option.

## Version requirements

| Package                  | Constraint for 1.0               |
| ------------------------ | -------------------------------- |
| `symfony/mercure-bundle` | `^0.5`                           |
| `symfony/mercure`        | `^0.8` (pulled in by the bundle) |
| PHP                      | `>= 8.2`                         |
| Symfony components       | `^6.4 \| ^7.3 \| ^8.0`           |
| Mercure hub              | `v1.0.0-alpha.3` or later        |

The Symfony constraint is worth reading twice: it is `^6.4 | ^7.3 | ^8.0`, not `^6.4 | ^7.0`. **Symfony 7.0 through 7.2 cannot install the bundle** — an app on 7.1 has to move to 7.3 or later before it can speak 1.0.

```console
# Version requirements
composer require symfony/mercure-bundle:^0.5
```

## Switch a hub to the 1.0 protocol

`protocol_version` is per hub, and it **defaults to `0.x`**. Upgrading the bundle therefore changes nothing on its own: an app pointed at a 1.0 hub keeps minting legacy `mercure`-claim tokens, and the hub rejects them with `401 invalid_token`. Set it explicitly:

```yaml
# config/packages/mercure.yaml
mercure:
  hubs:
    default:
      url: "%env(MERCURE_URL)%"
      public_url: "%env(MERCURE_PUBLIC_URL)%"
      protocol_version: "1.0"
      jwt:
        secret: "%env(MERCURE_JWT_SECRET)%"
        publish: ["*"]
        claims:
          iss: "%env(MERCURE_JWT_ISSUER)%"
          sub: "my-app"
          client_id: "my-app"
```

`protocol_version` drives three things: the claim shape built from `jwt.secret`, the default `cookie_name`, and how the `mercure()` Twig function interprets matcher-typed topics.

### The claims the bundle requires

[RFC 9068](https://www.rfc-editor.org/rfc/rfc9068) access tokens must carry `iss`, `sub` and `client_id`, so under `protocol_version: "1.0"` with `jwt.secret` or `jwt.jwks_uri` the bundle **refuses to compile the container** without all three:

```text
The "mercure.hubs.default.jwt.claims" option must define the "iss", "sub", "client_id" claim(s):
they are required by RFC 9068 access tokens when "protocol_version" is "1.0" and
"jwt.secret" or "jwt.jwks_uri" is used.
```

`iss` must match one of the [trusted issuers](../deployment/configuration.md#issuer-blocks) the hub declares in an `issuer` block (`MERCURE_TRUSTED_ISSUERS` on the official image). `sub` and `client_id` identify your application; the hub does not check their values, but it does require them to be present.

This does not apply to `jwt.value`, `jwt.provider` or a custom `jwt.factory` — if you mint tokens yourself, their shape is yours to get right.

### How `aud` is filled in

`aud` is exempt from the requirement above: when `jwt.claims.aud` is unset, the bundle defaults it to the hub's `public_url`, falling back to `url`.

That default is right whenever the browser and the app reach the hub at the same address. It is wrong in the common Docker layout where the app publishes through an internal `url` (`http://mercure/.well-known/mercure`) while the browser uses a public one, because a 1.0 hub derives its expected audience from the request it receives. Either pin the hub's [`resource_identifier`](../deployment/configuration.md) to the public URL, or set `aud` explicitly.

### Signing keys from a JWKS endpoint

`jwt.jwks_uri` fetches the signing key from a JSON Web Key Set instead of using a shared secret. It requires `protocol_version: "1.0"` and the `web-token/jwt-library` package; the bundle throws if the package is missing.

```yaml
# Signing keys from a JWKS endpoint
mercure:
  hubs:
    default:
      url: "%env(MERCURE_URL)%"
      protocol_version: "1.0"
      jwt:
        jwks_uri: "https://auth.example.com/.well-known/jwks.json"
        key_id: "publisher-2026" # required when the set holds several matching keys
        algorithm: "RS256"
        claims:
          iss: "https://auth.example.com"
          sub: "my-app"
          client_id: "my-app"
```

```console
# Signing keys from a JWKS endpoint
composer require web-token/jwt-library
```

`jwt.secret` and `jwt.jwks_uri` are mutually exclusive. Point the hub at the same key set with an issuer's [`jwks_uri`](../deployment/configuration.md#jwt-validation-via-jwks).

### `jwt.algorithm` has two spellings

`jwt.algorithm` has no single default, because the two token factories name algorithms differently:

| With           | Factory           | Names                                            | Default       |
| -------------- | ----------------- | ------------------------------------------------ | ------------- |
| `jwt.secret`   | `LcobucciFactory` | `hmac.sha256`, `hmac.sha384`, …                  | `hmac.sha256` |
| `jwt.jwks_uri` | `WebTokenFactory` | JWA names: `HS256`, `RS256`, `PS256`, `EdDSA`, … | `HS256`       |

Carrying an algorithm across from `secret` to `jwks_uri` without renaming it is a common trip-up.

### Cookie name

`cookie_name` defaults to a value computed from `protocol_version`: `mercureAuthorization` on `0.x`, `__Secure-mercure_access_token` on `1.0`. The `__Secure-` prefix requires HTTPS, so plain-HTTP local development needs a prefix-less name on **both** sides — `cookie_name` in the bundle and [`cookie_name`](../deployment/configuration.md) on the hub (the hub's `playground` directive already drops the prefix).

## Subscribing from the browser

The hub's side of 1.0 is the part your JavaScript sees: `topic=` is gone, replaced by `match=` and `match_urlpattern=`. See [Topics and matchers](../concepts/topics-and-matchers.md) and the [upgrade guide](../UPGRADE.md#migrate-your-subscribers).

API Platform publishes each update on the resource's absolute IRI, which is also the `@id` of the JSON-LD document it serves, so a subscriber can subscribe with what it already has:

```javascript
// Subscribing from the browser
const res = await fetch("https://api.example.com/books/42");
const book = await res.json();

const hub = res.headers.get("Link").match(/<([^>]+)>;\s*rel="?mercure"?/)[1];

const url = new URL(hub);
url.searchParams.append("match", new URL(book["@id"], res.url).toString());

new EventSource(url, { withCredentials: true });
```

`withCredentials` sends the authorization cookie; see [Authorization](../concepts/authorization.md#cookies-in-detail). To follow a collection rather than one resource, use a pattern:

```javascript
// Subscribing from the browser
url.searchParams.append(
  "match_urlpattern",
  "https://api.example.com/books/:id",
);
```

Both `symfony/mercure`'s `Discovery` and API Platform advertise the hub with `rel="mercure"` only, so the topic comes from the document (`@id`) or from the request URL — see [Discovery](../concepts/discovery.md).

## Running a 1.0 hub next to a Symfony app

Any hub from [Installation](../getting-started/installation.md) works. Two things to know if the app is built on FrankenPHP, as the [symfony-docker](https://github.com/dunglas/symfony-docker) and API Platform distributions are:

- **FrankenPHP's embedded hub is 0.x.** FrankenPHP 1.12.7 pins `github.com/dunglas/mercure v0.24.2`, so `mercure_publish()` and the hub it serves speak 0.x no matter which bundle version the app uses.
- **Building a 1.0 FrankenPHP binary** means compiling the 1.0 Caddy module in yourself. This is the [custom-build recipe from the FrankenPHP docs](https://frankenphp.dev/docs/docker/) with the Mercure module pinned:

```dockerfile
# Running a 1.0 hub next to a Symfony app
FROM dunglas/frankenphp:1-builder-php8.3 AS builder

COPY --from=caddy:2.11.4-builder /usr/bin/xcaddy /usr/bin/xcaddy

RUN CGO_ENABLED=1 \
    XCADDY_SETCAP=1 \
    XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
    CGO_CFLAGS=$(php-config --includes) \
    CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
    xcaddy build \
      --output /usr/local/bin/frankenphp \
      --with github.com/dunglas/frankenphp=./ \
      --with github.com/dunglas/frankenphp/caddy=./caddy/ \
      --with github.com/dunglas/mercure/caddy@v1.0.0-alpha.3
```

The `=./` replacements point `xcaddy` at the FrankenPHP sources the builder image already carries. Two versions have to line up for this to build at all: the builder image must ship Go 1.26 or later, the floor `mercure/caddy@v1.0.0-alpha.3` declares, and the Caddy version FrankenPHP pins must be the one that module expects (2.11.4 at the time of writing). See [Custom Caddy build](../getting-started/installation.md#custom-caddy-build).

The simpler alternative is to stop embedding: run the [official hub image](../deployment/docker.md) as its own service and point `MERCURE_URL` at it.

## Upgrade checklist

1. `composer require symfony/mercure-bundle:^0.5` — move to Symfony 7.3+ first if you are on 7.0–7.2.
2. Add `protocol_version: '1.0'` to each hub that talks to a 1.0 hub. Without it nothing changes.
3. Add `jwt.claims` with `iss`, `sub` and `client_id`. The container will not compile otherwise.
4. Make `iss` match the hub's trusted issuer, and check `aud` resolves to the URL the hub derives.
5. Replace `topic=` with `match=` / `match_urlpattern=` in every subscriber, and `{id}` templates with `:id` patterns.
6. If you rely on cookies over plain HTTP, set a prefix-less `cookie_name` on both sides.
7. Confirm the hub really is 1.0 — an embedded FrankenPHP hub is not.

## Next steps

- [Upgrade guide](../UPGRADE.md): the full 0.x to 1.0 migration, including tokens minted by hand.
- [Authorization](../concepts/authorization.md): what the hub validates in an access token.
- [Configuration](../deployment/configuration.md): `issuer` blocks, `resource_identifier`, `cookie_name`.
- [Awesome Mercure](awesome.md): the other framework integrations.
