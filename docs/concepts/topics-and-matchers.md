---
title: "Mercure topics and matchers: exact and URL Pattern matching"
description: "How subscribers select topics in Mercure with the two matcher types, Exact and URL Pattern, in both subscribe query parameters and authorization details."
---

# Topics and matchers

A **topic** is the address of an update. A **matcher** is the rule a subscriber uses to say which topics it cares about. Mercure defines two matcher types, `Exact` and `URLPattern`; every hub supports both. Pick the one that fits the shape of your data.

> **Upgrading from 0.x?** The subscriber query parameter changed from `topic=` to `match=` (exact) or `matchURLPattern=` (templated), and URI Templates are replaced by [URL Patterns](https://urlpattern.spec.whatwg.org). The Regexp, CEL, and URI Template matcher types are gone. Authorization claims are now `authorization_details` objects, not bare strings. Full details: [Upgrade guide](../UPGRADE.md#10-from-0x).

## Topics

Topics are arbitrary strings. Anything works:

- `https://example.com/books/42`
- `chat-room-1234`
- `urn:uuid:e1ee88e2-532a-4d6f-ba70-f0f8bd584022`
- `tenant:acme/orders/new`

### Why URLs are the recommended identifier

URLs (more precisely, [URIs](https://www.rfc-editor.org/rfc/rfc3986)) are the web's native identifier standard. Using them as Mercure topics is a best practice for the same reasons REST uses URIs to name resources:

- **Resources already have URLs.** If your application exposes `https://example.com/books/42`, that URL already identifies the book. Reusing it as the Mercure topic means subscribers and publishers reference the same thing in the same way, end to end.
- **Globally unique by construction.** A URL is unique within its domain and unique across domains. You don't need to coordinate a separate naming scheme between services.
- **Hypermedia-friendly.** URLs compose with `Link: rel="self"`, `JSON-LD` `@id`, Atom `<id>`, ActivityPub, OpenAPI, and every other web standard that already names things by URL. The same URL drops into a `fetch()`, an `<a href>`, a Mercure `match=`, and a database join.
- **Tooling understands them.** Browsers, log aggregators, IDEs, and the URL Pattern matcher all parse and validate URLs out of the box. Path-based routing and per-segment matching come for free.
- **REST principle.** A topic is a resource that publishers update and subscribers observe. Resources have URIs. Naming the topic with the resource's URI keeps the protocol uniform with the rest of your HTTP surface.

When you control the resource the update is about, the natural topic is its canonical URL.

### When to use a non-URL identifier

URLs aren't mandatory. Slugs, UUIDs, custom URN schemes, or any other string can be a topic. Use them when the thing being broadcast doesn't have a meaningful URL:

- Ephemeral or in-memory channels: `chat-room-1234`, `lobby:42`.
- Domain identifiers that aren't web-addressable: `urn:uuid:...`, `did:...`.
- Internal namespaced events from a single service: `tenant:acme/orders/new`.

Subscribers match these with `match` (full-string comparison). The hub doesn't care about the scheme; it treats topics as opaque strings.

Pick a scheme up front and stick to it. URLs are usually the right default; reach for a custom scheme only when there's no URL that names the thing you're broadcasting.

## Subscribing with matchers

A subscriber sends one or more `match*` query parameters when opening the SSE connection:

```text
# Subscribing with matchers
GET /.well-known/mercure?match=https://example.com/books/1&matchURLPattern=https://example.com/users/:id HTTP/2
```

The parameter name encodes the matcher type: bare `match` selects the default `Exact` type (`matchExact` is the explicit spelling), and `matchURLPattern` selects the `URLPattern` type. Parameter names are **case-sensitive**; any other name under the reserved `match` prefix is rejected with `400 Bad Request`, so a typo fails loudly instead of silently matching nothing. The subscriber receives every update whose topic matches **at least one** of the parameters.

| Matcher     | Query parameter              | Use it for                            |
| ----------- | ---------------------------- | ------------------------------------- |
| Exact       | `match` (alias `matchExact`) | Specific resources, fixed identifiers |
| URL Pattern | `matchURLPattern`            | Families of URLs (`/books/:id`)       |

## Exact matching with the `match` parameter

Case-sensitive string comparison. This is the matcher type the spec mandates every hub support, and the default when you use the bare `match` parameter.

```javascript
// Exact matching with the match parameter
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append("match", "https://example.com/books/1");
url.searchParams.append("match", "https://example.com/users/42");
new EventSource(url);
```

The connection above receives updates published with `topic=https://example.com/books/1` or `topic=https://example.com/users/42` (and nothing else).

## URL Pattern matchers

[URL Patterns](https://urlpattern.spec.whatwg.org) are the WHATWG standard used by service workers and modern routers. They're the way to subscribe to a family of URLs.

```javascript
// URL Pattern matchers
url.searchParams.append("matchURLPattern", "https://example.com/books/:id");
url.searchParams.append(
  "matchURLPattern",
  "https://example.com/users/:id/orders",
);
url.searchParams.append(
  "matchURLPattern",
  "https://example.com/feed/:type(news|alerts)",
);
```

URL Patterns understand:

- Named groups: `:id`
- Wildcards: `*`
- Regex constraints inside groups: `:type(news|alerts)`
- Optional segments: `/items{/:tail}?`

Patterns can be **absolute** (`https://example.com/...`) or **relative** to the hub URL (`/.well-known/mercure/subscriptions/:matchType/:match/:subscriber`). Relative patterns are resolved against the hub's `public_url` and are useful for [subscribing to subscription events](active-subscriptions.md), where the hub itself is the publisher. Matching is case-sensitive; `ignoreCase` is never enabled.

A topic matches a URL Pattern if the URL Pattern accepts the topic string as a URL.

> **URL Pattern playground.** The browser ships `URLPattern` natively. You can prototype patterns in the devtools console: `new URLPattern("https://example.com/books/:id").test("https://example.com/books/42")`.

## Combining matchers

A subscription with several `match*` parameters is a logical OR. There is no way to express AND inside a single subscription.

```javascript
// Combining matchers
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append("match", "https://example.com/site/announcement");
url.searchParams.append(
  "matchURLPattern",
  "https://example.com/users/:id/notifications",
);
new EventSource(url);
```

This subscriber receives:

- exactly the announcement topic, **or**
- any user-notifications URL.

## Authorization details use the same matcher types

The hub uses matchers in two places: at subscription time (which topics does the client want?) and at authorization time (which topics is the client _allowed to use_?). Both share the same two matcher types.

In an access token, each `mercure` entry of the `authorization_details` claim holds a `topics` array of matcher objects:

```jsonc
// Authorization details use the same matcher types
{
  "authorization_details": [
    {
      "type": "mercure",
      "actions": ["subscribe"],
      "topics": [
        { "match": "https://example.com/users/42" },
        {
          "match": "https://example.com/users/42/:resource",
          "matchType": "URLPattern",
        },
      ],
    },
    { "type": "mercure", "actions": ["publish"], "topics": [{ "match": "*" }] },
  ],
}
```

`matchType` is case-sensitive and defaults to `Exact` when omitted. The reserved value `*` (with `matchType: "Exact"` or omitted) means "every topic." Full details and examples in [Authorization](authorization.md).

## How matching works on the publish side

A publisher posts an update with exactly one `topic`. The hub runs every connected subscriber's matchers against that topic. Public updates go to every subscriber whose matchers hit. Private updates additionally require that the subscriber's token grants `subscribe` on that topic. See [Publishing](publishing.md) and [Authorization](authorization.md) for the full path.

## Picking a matcher

A short rule of thumb:

- One specific resource (or a non-URL identifier) -> **Exact**.
- All resources of a type -> **URL Pattern**.
