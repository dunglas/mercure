---
title: "Discovering a Mercure hub and its authorization requirements"
description: "How clients find the Mercure hub with a Link header and read its OAuth 2.0 protected resource metadata (RFC 9728) to learn where to obtain an access token."
---

# Mercure discovery

A client needs the hub URL and, for private updates, an access token. Resource `Link` headers advertise the hub. OAuth protected resource metadata describes its audience and configured authorization servers. The client still needs support for Mercure topic grants.

## Finding the hub

A resource advertises its hub with a [Web Linking](https://www.rfc-editor.org/rfc/rfc8288) `Link` header (or the equivalent HTML `<link>` element) carrying `rel="mercure"`:

```http
GET /books/42 HTTP/2
Host: example.com

HTTP/2 200
Link: <https://hub.example.com/.well-known/mercure>; rel="mercure"
Content-Type: application/json

{ "@id": "/books/42", "title": "..." }
```

The client parses the header, takes the URL with `rel="mercure"`, appends its `match*` query parameters, and opens an `EventSource`. Reusing your existing API responses to carry the link keeps subscribers and publishers pointing at the same hub.

A resource's own URL is normally its topic too, so `res.url` is all a subscriber needs to `match` on.

## Content negotiation on the topic

The one case that needs more is content negotiation: the same conceptual resource served under several representations, for example by language (`Accept-Language`) or by format (`Accept`). The hub can't see which representation the subscriber's browser would negotiate, so it can't resolve those variants back to one topic on its own.

For that case, the publisher **may** include a second link, `rel="self"`, holding the canonical URL you want subscribers to use as the topic. Point every representation's `rel="self"` at the same URL to share one topic across variants, or give each its own to keep them separate:

```http
# Content negotiation on the topic
GET /books/42 HTTP/2
Host: example.com
Accept-Language: fr

HTTP/2 200
Link: <https://hub.example.com/.well-known/mercure>; rel="mercure"
Link: </books/42>; rel="self"
Content-Language: fr
```

```javascript
// Content negotiation on the topic
const res = await fetch("https://example.com/books/42", {
  headers: { "Accept-Language": "fr" },
});
const links = res.headers.get("Link");

const hub = links.match(/<([^>]+)>;\s*rel="?mercure"?/)[1];
const self = links.match(/<([^>]+)>;\s*rel="?self"?/)?.[1] ?? res.url;

const url = new URL(hub);
url.searchParams.append("match", new URL(self, res.url).toString());
new EventSource(url);
```

Outside content negotiation, skip `rel="self"` and match on `res.url` directly, as in [Subscribing](subscribing.md#discovering-the-mercure-hub-via-link-header).

## Protected resource metadata

The hub is an OAuth 2.0 protected resource, so it publishes [OAuth 2.0 Protected Resource Metadata](https://www.rfc-editor.org/rfc/rfc9728). For a hub at `https://hub.example.com/.well-known/mercure`, the metadata lives at:

```text
https://hub.example.com/.well-known/oauth-protected-resource/.well-known/mercure
```

```json
{
  "resource": "https://hub.example.com/.well-known/mercure",
  "bearer_methods_supported": ["header"],
  "authorization_details_types_supported": [
    "https://mercure.rocks/authorization-detail"
  ],
  "authorization_servers": ["https://auth.example.com"],
  "mercure_cookie": "__Secure-mercure_access_token"
}
```

Members:

- `resource`: the hub's resource identifier. This is the value a token's `aud` claim must contain (see [Authorization](authorization.md)).
- `bearer_methods_supported`: the [RFC 6750](https://www.rfc-editor.org/rfc/rfc6750) presentation methods the hub accepts: `header` (the `Authorization` header). The `access_token` query parameter is not accepted ([RFC 9700](https://www.rfc-editor.org/rfc/rfc9700)).
- `authorization_details_types_supported`: always contains `https://mercure.rocks/authorization-detail`, the [RFC 9396](https://www.rfc-editor.org/rfc/rfc9396) authorization detail type this hub understands.
- `authorization_servers` (optional): the issuer identifiers of the authorization servers that mint tokens for this hub. A client uses these to locate the server, run an OAuth 2.0 flow, and obtain an access token. Advertise an issuer by adding `authorization_server` inside its `issuer` block.
- `mercure_cookie` (optional): the name of the cookie in which the hub also accepts the token. A cookie is not an RFC 6750 method, so it has its own member rather than appearing in `bearer_methods_supported`.

The hub serves this document only when it validates tokens (a pure-anonymous hub has nothing to advertise). The `jwks_uri` member is intentionally omitted: the hub hosts no JWKS endpoint, and the separate publisher and subscriber key sets can't be expressed as one `jwks_uri`. To validate tokens against an external key set, point an issuer's verifier at it with `jwks_uri` (see [Configuration](../deployment/configuration.md#jwt-validation-via-jwks)).

## How the pieces fit together

When a client hits an operation that needs a token without one, the hub answers `401` with a bare `WWW-Authenticate: Bearer` challenge that includes a `resource_metadata` parameter pointing at the document above:

```http
HTTP/2 401
WWW-Authenticate: Bearer resource_metadata="https://hub.example.com/.well-known/oauth-protected-resource/.well-known/mercure"
```

A client that doesn't yet have a token follows that parameter, reads `authorization_servers`, obtains a token from the named authorization server, and retries. See [Authorization](authorization.md) for the full set of [RFC 6750](https://www.rfc-editor.org/rfc/rfc6750) responses.
