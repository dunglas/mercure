---
title: "Publishing Real-Time Updates to a Mercure Hub"
description: "Send POST requests to publish public and private updates to a Mercure hub, attach alternate topics, and authorize publishers with a JWT."
---

# Publishing

A publication is an HTTP `POST` to the hub with a form-encoded body:

```http
# Publishing
POST /.well-known/mercure HTTP/1.1
Host: hub.example.com
Authorization: Bearer <publisher JWT>
Content-Type: application/x-www-form-urlencoded

topic=https%3A%2F%2Fexample.com%2Fbooks%2F1&data=%7B%22status%22%3A%22checked+out%22%7D
```

The hub fans the update out to every subscriber whose matchers hit at least one of the publication's topics, then returns the event ID it assigned.

## Mercure Publish Form Fields

| Field | Required | Description |
| --- | --- | --- |
| `topic` | Yes | Identifier of the topic. Repeat for alternate identifiers (see below). |
| `data` | No | Payload of the update. Anything you want — JSON, HTML, JSON Patch, plain text. |
| `private` | No | If present, the update is private. The hub only delivers it to subscribers authorized for one of the topics. |
| `id` | No | Custom event ID. Must not start with `#`. The hub assigns one if you don't. |
| `type` | No | Custom SSE `event` type. Defaults to `message`. |
| `retry` | No | Reconnection time hint, in milliseconds. |

The body is `application/x-www-form-urlencoded` — every field is URL-encoded.

## Mercure Publish Examples

### Publishing to Mercure with curl

```console
# Publishing to Mercure with curl
curl -X POST https://hub.example.com/.well-known/mercure \
  -H "Authorization: Bearer $JWT" \
  -d 'topic=https://example.com/books/1' \
  -d 'data={"status": "checked out"}'
```

### Publishing to Mercure from Node.js

```javascript
// Publishing to Mercure from Node.js
await fetch("https://hub.example.com/.well-known/mercure", {
  method: "POST",
  headers: {
    Authorization: `Bearer ${jwt}`,
    "Content-Type": "application/x-www-form-urlencoded",
  },
  body: new URLSearchParams({
    topic: "https://example.com/books/1",
    data: JSON.stringify({ status: "checked out" }),
  }),
});
```

### Publishing to Mercure from Python

```python
# Publishing to Mercure from Python
import requests

requests.post(
    "https://hub.example.com/.well-known/mercure",
    headers={"Authorization": f"Bearer {jwt}"},
    data={"topic": "https://example.com/books/1", "data": '{"status": "checked out"}'},
)
```

### Publishing to Mercure from PHP with Symfony

The [Symfony Mercure component](https://symfony.com/doc/current/mercure.html) wraps the protocol:

```php
// Publishing to Mercure from PHP with Symfony
use Symfony\Component\Mercure\HubInterface;
use Symfony\Component\Mercure\Update;

public function __invoke(HubInterface $hub) {
    $hub->publish(new Update(
        'https://example.com/books/1',
        json_encode(['status' => 'checked out']),
    ));
}
```

## Alternate topics

A single update can carry multiple topics. The first `topic` is the **canonical** identifier; the rest are **alternates**. The hub delivers the update to any subscriber whose matchers hit one of them.

```console
# Alternate topics
curl -X POST https://hub.example.com/.well-known/mercure \
  -H "Authorization: Bearer $JWT" \
  -d 'topic=https://example.com/books/1' \
  -d 'topic=urn:uuid:e1ee88e2-532a-4d6f-ba70-f0f8bd584022' \
  -d 'data={"status": "checked out"}'
```

Use alternates when the same logical update can be referred to by several names — a URL and a UUID, an English URL and a translated URL, the canonical version and a per-tenant version. They're also the mechanism that powers per-user authorization for shared resources; see [Authorization](authorization.md#per-user-authorization-on-shared-topics).

## Public vs. private updates

Without the `private` field, an update is **public**: the hub sends it to every subscriber whose matchers hit, regardless of whether they presented a JWT.

With `private=on` (the value can be anything; `on` is the convention), the update is **private**: a subscriber receives it only if its `mercure.subscribe` JWT claim covers at least one of the update's topics.

```console
# Public — anyone subscribed to this topic gets it
curl -X POST $HUB -H "Authorization: Bearer $JWT" \
  -d 'topic=https://example.com/news/latest' \
  -d 'data=...'

# Private — only authorized subscribers get it
curl -X POST $HUB -H "Authorization: Bearer $JWT" \
  -d 'topic=https://example.com/users/42/inbox' \
  -d 'data=...' \
  -d 'private=on'
```

If you want updates on a topic to be visible only to authorized subscribers, **mark them private**. The hub does not infer privacy from the topic URL.

## Authorization

The publisher's JWT must contain a `mercure.publish` claim with at least one matcher that covers every topic in the publication. Otherwise the hub returns `403 Forbidden`.

```jsonc
// Authorization
{
  "mercure": {
    "publish": [
      { "match": "https://example.com/books/:id", "matchType": "URLPattern" }
    ]
  }
}
```

A token with `[{ "match": "*" }]` can publish to anything. See [Authorization](authorization.md#publishers) for details.

## What the hub returns

```http
# What the hub returns
200 OK
Content-Type: text/plain

urn:uuid:e1ee88e2-532a-4d6f-ba70-f0f8bd584022
```

The body is the event ID the hub assigned to the update. Store it if you need to:

- Replay from this point later via `Last-Event-ID`.
- Correlate the update with a downstream system (e.g. a write to your own database).

If you provided your own `id`, the hub uses it as-is (subject to a few constraints noted in the spec) and echoes it back.

## When to Publish Mercure Updates from Your Application

Publish from the same code path that mutates the underlying state. The simplest pattern, in pseudocode:

```text
# When to Publish Mercure Updates from Your Application
function updateBook(id, data):
    db.update(id, data)
    hub.publish(
        topic="https://example.com/books/" + id,
        data=json(data),
    )
```

For stricter delivery guarantees (every state change reaches the hub even if the publish call fails), wrap both writes in a transactional outbox: persist the update next to the row, and have a worker ship it to the hub. The Mercure hub itself is reliable; the network between your app and the hub is what you need to defend against.

## Embedded publishing (no external hub)

The Mercure protocol does not require an external hub. An application that already terminates HTTP/2 connections can speak the protocol directly: write SSE bytes to subscribers, validate JWTs, run matchers. This is unusual outside of frameworks that ship their own hub (FrankenPHP, for instance), but the spec allows it.

For everyone else, run the hub.

## Next Steps for Mercure Publishing

- [Authorization](authorization.md) — minting JWTs that pass validation.
- [Active subscriptions](active-subscriptions.md) — knowing who's connected.
- [Reconnection and history](reconnection-and-history.md) — making sure subscribers don't miss updates.
