---
title: "Mercure Topics, Matchers, and Subscription Filters"
description: "How subscribers select topics in Mercure 1.0 with exact match, URL Pattern, regular expression, CEL, and URI Template matchers."
---

# Topics and Matchers

A **topic** is the address of an update. A **matcher** is the rule a subscriber uses to say which topics it cares about. Mercure 1.0 supports several matcher types — pick the one that fits the shape of your data.

> **Upgrading from 0.x?** The query parameter for subscribers changed from `topic=` to `match=` (exact) or `matchURLPattern=` (templated). URI Templates are still supported as `matchURITemplate=` but no longer the default. JWT claims are now objects, not strings. Full details: [Upgrade guide](../UPGRADE.md#10-from-0x).

## Topics

Topics are arbitrary strings. The protocol recommends URLs because they compose well with hypermedia APIs (REST, JSON-LD, Atom), but anything works:

- `https://example.com/books/42`
- `chat-room-1234`
- `urn:uuid:e1ee88e2-532a-4d6f-ba70-f0f8bd584022`
- `tenant:acme/orders/new`

Pick a scheme up front and stick to it. URLs are usually the right default; use a custom scheme only when there's no URL that names the thing you're broadcasting.

## Subscribing with matchers

A subscriber sends one or more `match*` query parameters when opening the SSE connection:

```text
# Subscribing with matchers
GET /.well-known/mercure?match=https://example.com/books/1&matchURLPattern=https://example.com/users/:id HTTP/2
```

Each parameter starts with the literal string `match`, followed by the matcher type (case-insensitive: `matchURLPattern`, `matchurlpattern`, and `MATCHURLPATTERN` are equivalent). The subscriber receives every update whose topic matches **at least one** of the parameters.

| Matcher | Query parameter | Required by hubs | Use it for |
| --- | --- | --- | --- |
| Exact | `match` (alias `matchExact`) | **MUST** | Specific resources, fixed identifiers |
| URL Pattern | `matchURLPattern` | **SHOULD** | Families of URLs (`/books/:id`) |
| Regular expression | `matchRegexp` | **SHOULD** | Strings that aren't URLs (rooms, slugs) |
| Common Expression Language | `matchCEL` | **MAY** | Boolean logic over multiple topic fields |
| URI Template | `matchURITemplate` | **MAY** | Compatibility with 0.x clients |

If a subscriber asks for a matcher type the hub doesn't implement, the hub responds `501 Not Implemented`.

## Exact Matching with the `match` Parameter

Case-sensitive string comparison. The matcher type the spec mandates every hub support.

```javascript
// Exact Matching with the match Parameter
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append("match", "https://example.com/books/1");
url.searchParams.append("match", "https://example.com/users/42");
new EventSource(url);
```

The connection above receives updates published with `topic=https://example.com/books/1` or `topic=https://example.com/users/42` (and nothing else).

## URL Pattern Matchers

[URL Patterns](https://urlpattern.spec.whatwg.org) are the WHATWG specification used by service workers and modern routers. They're the recommended way to subscribe to a family of URLs.

```javascript
// URL Pattern Matchers
url.searchParams.append("matchURLPattern", "https://example.com/books/:id");
url.searchParams.append("matchURLPattern", "https://example.com/users/:id/orders");
url.searchParams.append("matchURLPattern", "https://example.com/feed/:type(news|alerts)");
```

URL Patterns understand:

- Named groups: `:id`
- Wildcards: `*`
- Regex constraints inside groups: `:type(news|alerts)`
- Optional segments: `/items{/:tail}?`

Patterns can be **absolute** (`https://example.com/...`) or **relative** to the hub URL (`/.well-known/mercure/subscriptions/Exact/:topic/:subscriber`). Relative patterns are useful for [subscribing to subscription events](active-subscriptions.md), where the hub itself is the publisher.

A topic matches a URL Pattern if the URL Pattern accepts the topic string as a URL.

> **URL Pattern playground.** The browser ships `URLPattern` natively. You can prototype patterns in the devtools console: `new URLPattern("https://example.com/books/:id").test("https://example.com/books/42")`.

## Regular Expression Matchers (I-Regexp)

`matchRegexp` takes an [I-Regexp](https://www.rfc-editor.org/rfc/rfc9485) regular expression — the interoperable subset that JSON Schema, XPath, and most modern engines agree on.

```javascript
// Regular Expression Matchers (I-Regexp)
url.searchParams.append("matchRegexp", "^chat-room-[0-9]+$");
url.searchParams.append("matchRegexp", "^tenant:acme/.*/error$");
```

Reach for regular expressions when your topics aren't URLs, or when URL Patterns can't express the thing you need (negative lookaheads, anchors past path segments).

## Common Expression Language (CEL)

[CEL](https://cel.dev/) is a small, sandboxed expression language used by Kubernetes, gRPC, and Cloud IAM. The hub passes a `topics` array to the expression — index `0` is the canonical topic, the rest are alternates. The expression must return a boolean.

```javascript
// Common Expression Language (CEL)
url.searchParams.append(
  "matchCEL",
  "topics[0].startsWith('https://example.com/books/') && topics.exists(t, t.contains('lang=en'))"
);
```

CEL is the most expressive matcher but also the most expensive. Hubs that implement it apply an evaluation cost limit and treat over-budget expressions as `false`. Use it when neither URL Patterns nor regular expressions can express the predicate, not as the default.

## URI Template Matchers (Backward Compatibility)

[URI Templates](https://www.rfc-editor.org/rfc/rfc6570) (`/books/{id}`) were the templating language of choice in Mercure 0.x. They're still supported via `matchURITemplate` for backward compatibility, but new code should use URL Patterns — they handle URLs better and are natively understood by browsers.

```javascript
// URI Template Matchers (Backward Compatibility)
url.searchParams.append("matchURITemplate", "https://example.com/books/{id}");
```

## Combining matchers

A subscription with several `match*` parameters is a logical OR. There is no way to express AND inside a subscription — if you need that, use CEL.

```javascript
// Combining matchers
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append("match", "https://example.com/site/announcement");
url.searchParams.append("matchURLPattern", "https://example.com/users/:id/notifications");
url.searchParams.append("matchRegexp", "^chat-room-(42|99)$");
new EventSource(url);
```

This subscriber receives:
- exactly the announcement topic, **or**
- any user-notifications URL, **or**
- the chat rooms 42 and 99.

## Authorization claims use the same matcher types

The hub uses matchers in two places: at subscription time (which topics does the client want?) and at authorization time (which topics is the client *allowed to use*?). Both share the same matcher vocabulary.

In a JWT, the `mercure.subscribe` and `mercure.publish` claims hold an array of objects:

```jsonc
// Authorization claims use the same matcher types
{
  "mercure": {
    "subscribe": [
      { "match": "https://example.com/users/42" },
      { "match": "https://example.com/users/42/:resource", "matchType": "URLPattern" }
    ],
    "publish": [
      { "match": "*" }
    ]
  }
}
```

`matchType` defaults to `"Exact"` if omitted. The reserved value `"*"` (with `matchType: "Exact"` or omitted) means "every topic." Full details and examples in [Authorization](authorization.md).

## How matching works on the publish side

When a publisher posts an update with a `topic` (and optionally several alternate `topic` values), the hub runs every connected subscriber's matchers against the topic list. Public updates go to every subscriber whose matchers hit. Private updates additionally check that the subscriber's `mercure.subscribe` claim matches at least one of the topics. See [Publishing](publishing.md) and [Authorization](authorization.md) for the full path.

## Picking a matcher

A short rule of thumb:

- One specific resource → **Exact**.
- All resources of a type → **URL Pattern**.
- Topics that aren't URLs → **Regular expression**.
- Multi-condition or multi-topic predicates → **CEL**.
- Migrating from 0.x and don't want to rewrite patterns yet → **URI Template**.
