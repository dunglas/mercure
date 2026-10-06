---
title: "Mercure topics and matchers: exact and URL Pattern matching"
description: "How subscribers select topics in Mercure with the two matcher types, Exact and URL Pattern, in both subscribe query parameters and authorization details."
---

# Mercure topics and matchers

A **topic** identifies the resource or channel an update concerns. A **matcher** selects topics for a subscription or authorization grant. Mercure defines two matcher types: `exact` and `urlpattern`.

> **Upgrading from 0.x?** The subscriber query parameter changed from `topic=` to `match=` (exact) or `match_urlpattern=` (templated), and URI Templates are replaced by [URL Patterns](https://urlpattern.spec.whatwg.org). The URI Template matcher type is gone. Authorization claims are now `authorization_details` objects, not bare strings. Full details: [Upgrade guide](../UPGRADE.md#10-from-0x).

## Topics

Topics are non-empty strings. This hub limits each topic or matcher pattern to 4,096 bytes and reserves special values and hub-owned paths; see the [specification](../../spec/mercure.md). Examples:

- `https://example.com/books/42`
- `chat-room-1234`
- `urn:uuid:e1ee88e2-532a-4d6f-ba70-f0f8bd584022`
- `tenant:acme/orders/new`

## Subscribing with matchers

A subscriber sends between 1 and 100 `match*` query parameters in this hub when opening the SSE connection:

```text
GET /.well-known/mercure?match=https://example.com/books/1&match_urlpattern=https://example.com/users/:id HTTP/2
```

The parameter name encodes the matcher type: bare `match` selects the default `exact` type (`match_exact` is the explicit spelling), and `match_urlpattern` selects the `urlpattern` type. Parameter names are **case-sensitive**; any other name under the reserved `match` prefix is rejected with `400 Bad Request`, so a typo fails loudly instead of silently matching nothing. The subscriber receives every update whose topic matches **at least one** of the parameters.

| Matcher     | Query parameter               | Use it for                            |
| ----------- | ----------------------------- | ------------------------------------- |
| Exact       | `match` (alias `match_exact`) | Specific resources, fixed identifiers |
| URL Pattern | `match_urlpattern`            | Families of URLs (`/books/:id`)       |

## Exact matching with the `match` parameter

Case-sensitive string comparison. This is the matcher type the spec mandates every hub support, and the default when you use the bare `match` parameter.

```javascript
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append("match", "https://example.com/books/1");
url.searchParams.append("match", "https://example.com/users/42");
new EventSource(url);
```

This connection selects updates with either topic. Private updates also require a matching authorization grant.

## URL Pattern matchers

[URL Patterns](https://urlpattern.spec.whatwg.org) are the WHATWG standard used by service workers and modern routers. They're the way to subscribe to a family of URLs.

```javascript
url.searchParams.append("match_urlpattern", "https://example.com/books/:id");
url.searchParams.append(
  "match_urlpattern",
  "https://example.com/users/:id/orders",
);
url.searchParams.append(
  "match_urlpattern",
  "https://example.com/feed/:type(news|alerts)",
);
```

URL Patterns understand:

- Named groups: `:id`
- Wildcards: `*`
- Regular expression constraints inside groups: `:type(news|alerts)`
- Optional segments: `/items{/:tail}?`

Patterns can be **absolute** (`https://example.com/...`) or **relative** to the hub URL (`/.well-known/mercure/subscriptions/:match_type/:match/:subscriber`). Relative patterns are resolved against the hub's base URL (a `resource_identifier` ending in `/.well-known/mercure`, otherwise a synthetic base) and are useful for [subscribing to subscription events](active-subscriptions.md), where the hub itself is the publisher. Matching is case-sensitive; `ignoreCase` is never enabled.

A topic matches a URL Pattern if the URL Pattern accepts the topic string as a URL.

Test patterns in a browser that supports `URLPattern`: `new URLPattern("https://example.com/books/:id").test("https://example.com/books/42")`.

## Combining matchers

A subscription with several `match*` parameters is a logical OR. There is no way to express AND inside a single subscription.

```javascript
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append("match", "https://example.com/site/announcement");
url.searchParams.append(
  "match_urlpattern",
  "https://example.com/users/:id/notifications",
);
new EventSource(url);
```

This subscriber receives:

- exactly the announcement topic, **or**
- any user-notifications URL.

## Authorization details use the same matcher types

The hub uses matchers in two places: at subscription time (which topics does the client want?) and at authorization time (which topics is the client _allowed to use_?). Both share the same two matcher types.

In an access token, each Mercure entry of the `authorization_details` claim holds a `topics` array of matcher objects:

```json
{
  "authorization_details": [
    {
      "type": "https://mercure.rocks/authorization-detail",
      "actions": ["subscribe"],
      "topics": [
        { "match": "https://example.com/users/42" },
        {
          "match": "https://example.com/users/42/:resource",
          "match_type": "urlpattern"
        }
      ]
    },
    {
      "type": "https://mercure.rocks/authorization-detail",
      "actions": ["publish"],
      "topics": [{ "match": "*" }]
    }
  ]
}
```

`match_type` is case-sensitive and defaults to `exact` when omitted. The reserved value `*` (with `match_type: "exact"` or omitted) means "every topic." Full details and examples in [Authorization](authorization.md).

## How matching works on the publish side

A publisher posts an update with one or more `topic` fields; the first is the canonical topic, any others are [alternate topics](#alternate-topics). The hub runs every connected subscriber's matchers against all of the update's topics. Public updates go to every subscriber whose matchers hit any one of them. Private updates additionally require that the subscriber's token grants `subscribe` on at least one of them. See [Publishing](publishing.md) and [Authorization](authorization.md) for the full path.

## Alternate topics

A single update can carry more than one `topic` field. The first is the **canonical topic**, the primary identifier of the updated resource. Any others are **alternate topics**. The hub dispatches the update to every subscriber matching the canonical topic _or_ any alternate, and, for private updates, authorizes it the same way: a subscriber needs a `subscribe` grant on just one of the topics, not the canonical one specifically.

```console
curl -X POST "$HUB" -H "Authorization: Bearer $JWT" \
  --data-urlencode 'topic=https://example.com/books/1' \
  --data-urlencode 'topic=https://example.com/users/42/books/1' \
  --data-urlencode 'private=on' \
  --data-urlencode 'data=...'
```

Alternate topics let one update reach several audiences. For a private update, every alternate topic expands the possible audience. The publisher must be authorized for all topics. See [Per-user authorization on shared resources](authorization.md#per-user-authorization-on-shared-resources).

## Choosing topic identifiers

### Why URLs are the recommended identifier

URLs are the web's native identifiers. Using them as Mercure topics makes your real-time API fit the rest of your application:

- **Reuse the URLs you already have.** If `https://example.com/books/42` identifies a book in your API, use it as the topic too. Publishers and subscribers refer to the same resource.
- **Keep namespaces distinct.** Your domain separates your topics from those of other services without a central channel registry. Choose a consistent path for each resource.
- **Connect with web standards.** A resource URL can serve as a JSON-LD `@id`, an ActivityPub object ID, a link target, and a Mercure topic.
- **Subscribe to related resources.** URL Patterns select families such as `https://example.com/books/:id`, using the same paths your API exposes.

When the resource has a canonical URL, that's the natural topic. The hub does not fetch topic URLs, so a topic does not have to resolve to a page.

### When to use a non-URL identifier

URLs aren't mandatory. Slugs, UUIDs, custom URN schemes, or any other string can be a topic. Use them when the thing being broadcast doesn't have a meaningful URL:

- Ephemeral or in-memory channels: `chat-room-1234`, `lobby:42`.
- Domain identifiers that aren't web-addressable: `urn:uuid:...`, `did:...`.
- Internal namespaced events from a single service: `tenant:acme/orders/new`.

Subscribers match these with `match` (full-string comparison). The hub doesn't care about the scheme; it treats topics as opaque strings.

Use a consistent naming scheme across publishers, subscribers, and authorization grants.

## Picking a matcher

- One specific resource (or a non-URL identifier) -> **Exact**.
- All resources of a type -> **URL Pattern**.
