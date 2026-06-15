---
title: "Discovering a Mercure hub and its authorization requirements"
description: "How clients find the Mercure hub with a Link header and read its OAuth 2.0 protected resource metadata (RFC 9728) to learn where to obtain an access token."
---

# Discovery

A client needs two things before it can subscribe to private updates: the **URL of the hub**, and the **authorization requirements** of that hub. Mercure exposes both through standard mechanisms, so a generic OAuth 2.0 client library can discover them without Mercure-specific code.

## Finding the hub

A resource advertises its hub with a [Web Linking](https://www.rfc-editor.org/rfc/rfc8288) `Link` header (or the equivalent HTML `<link>` element) carrying `rel="mercure"`:

```http
# Finding the hub
GET /books/42 HTTP/2
Host: example.com

HTTP/2 200
Link: <https://hub.example.com/.well-known/mercure>; rel="mercure"
Content-Type: application/json

{ "@id": "/books/42", "title": "..." }
```

The client parses the header, takes the URL with `rel="mercure"`, appends its `match*` query parameters, and opens an `EventSource`. Reusing your existing API responses to carry the link keeps subscribers and publishers pointing at the same hub.

## Protected resource metadata

The hub is an OAuth 2.0 protected resource, so it publishes [OAuth 2.0 Protected Resource Metadata](https://www.rfc-editor.org/rfc/rfc9728). For a hub at `https://hub.example.com/.well-known/mercure`, the metadata lives at:

```text
# Protected resource metadata location
https://hub.example.com/.well-known/oauth-protected-resource/.well-known/mercure
```

```json
// GET /.well-known/oauth-protected-resource/.well-known/mercure
{
  "resource": "https://hub.example.com/.well-known/mercure",
  "bearer_methods_supported": ["header", "query"],
  "authorization_details_types_supported": ["mercure"],
  "authorization_servers": ["https://auth.example.com"],
  "mercure_cookie": true
}
```

Members:

- `resource`: the hub's resource identifier. This is the value a token's `aud` claim must contain (see [Authorization](authorization.md)).
- `bearer_methods_supported`: the [RFC 6750](https://www.rfc-editor.org/rfc/rfc6750) presentation methods the hub accepts: `header` (the `Authorization` header) and, when enabled, `query` (the `access_token` query parameter).
- `authorization_details_types_supported`: always contains `mercure`, the [RFC 9396](https://www.rfc-editor.org/rfc/rfc9396) authorization detail type this hub understands.
- `authorization_servers` (optional): the issuer identifiers of the authorization servers that mint tokens for this hub. A client uses these to locate the server, run an OAuth 2.0 flow, and obtain an access token. Configure them with the `authorization_servers` directive.
- `mercure_cookie` (optional): `true` when the hub also accepts the token in a cookie. A cookie is not an RFC 6750 method, so it has its own member rather than appearing in `bearer_methods_supported`.

The hub serves this document only when it validates tokens (a pure-anonymous hub has nothing to advertise). The `jwks_uri` member is intentionally omitted: the hub hosts no JWKS endpoint, and the separate publisher and subscriber key sets can't be expressed as one `jwks_uri`. To validate tokens against an external key set, point the hub at it with `publisher_jwks_url` / `subscriber_jwks_url` (see [Configuration](../deployment/configuration.md#jwt-validation-via-jwks)).

## How the pieces fit together

When a client hits an operation that needs a token without one, the hub answers `401` with a bare `WWW-Authenticate: Bearer` challenge that includes a `resource_metadata` parameter pointing at the document above:

```http
# Bearer challenge
HTTP/2 401
WWW-Authenticate: Bearer resource_metadata="https://hub.example.com/.well-known/oauth-protected-resource/.well-known/mercure"
```

A client that doesn't yet have a token follows that parameter, reads `authorization_servers`, obtains a token from the named authorization server, and retries. See [Authorization](authorization.md) for the full set of [RFC 6750](https://www.rfc-editor.org/rfc/rfc6750) responses.
