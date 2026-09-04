---
title: "Discovering a Mercure hub and its authorization requirements"
description: "How clients find the Mercure hub with a Link header and read its OAuth 2.0 protected resource metadata (RFC 9728) to learn where to obtain an access token."
---

# Discovery

A client needs three things before it can subscribe to private updates: the **URL of the hub**, the **canonical URL of the topic** it wants, and the **authorization requirements** of that hub. Mercure exposes all three through standard mechanisms, so a generic OAuth 2.0 client library can discover them without Mercure-specific code.

## Finding the hub

A resource advertises its hub with a [Web Linking](https://www.rfc-editor.org/rfc/rfc8288) `Link` header (or the equivalent HTML `<link>` element) carrying `rel="mercure"`:

```http
# Finding the hub
GET /books/42 HTTP/2
Host: example.com

HTTP/2 200
Link: <https://hub.example.com/.well-known/mercure>; rel="mercure"
Link: </books/42>; rel="self"
Content-Type: application/json

{ "@id": "/books/42", "title": "..." }
```

The client parses the header, takes the URL with `rel="mercure"`, appends its `match*` query parameters, and opens an `EventSource`. Reusing your existing API responses to carry the link keeps subscribers and publishers pointing at the same hub.

## Finding the topic

The same response says which topic to subscribe to. The publisher **may** include a second link, `rel="self"`, holding the canonical URL of the topic; when it is absent, the client falls back to the URL of the resource it just fetched:

```javascript
// Finding the topic
const res = await fetch("https://example.com/books/42");
const links = res.headers.get("Link");

const hub = links.match(/<([^>]+)>;\s*rel="?mercure"?/)[1];
const self = links.match(/<([^>]+)>;\s*rel="?self"?/)?.[1] ?? res.url;

const url = new URL(hub);
url.searchParams.append("match", new URL(self, res.url).toString());
new EventSource(url);
```

This is what makes a subscriber generic: it never has to know how the publisher builds its topic URLs. It matters most when the resource URL and the topic are not the same string — a resource served under several representations (`/books/42.jsonld`, `/books/42.html`) can point every one of them at one canonical topic, or give each its own. The protocol allows either, as long as `rel="self"` says which; it is also how content negotiation works on the topic, since the hub cannot pick a representation on the subscriber's behalf.

Publish on the topic the self link advertises, and use absolute URLs on both sides where you can; a relative value like `/books/42` is legal, but it then has to resolve the same way for the client that subscribes and for the hub that matches (see [`resource_identifier`](../deployment/configuration.md)).

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
# Bearer challenge
HTTP/2 401
WWW-Authenticate: Bearer resource_metadata="https://hub.example.com/.well-known/oauth-protected-resource/.well-known/mercure"
```

A client that doesn't yet have a token follows that parameter, reads `authorization_servers`, obtains a token from the named authorization server, and retries. See [Authorization](authorization.md) for the full set of [RFC 6750](https://www.rfc-editor.org/rfc/rfc6750) responses.
