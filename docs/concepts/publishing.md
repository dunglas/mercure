---
title: "Publishing real-time updates to a Mercure hub"
description: "Send POST requests to publish public and private updates to a Mercure hub and authorize publishers with an OAuth 2.0 access token."
---

# Publishing

A publication is an HTTP `POST` to the hub with a form-encoded body:

```http
# Publishing
POST /.well-known/mercure
Host: hub.example.com
Authorization: Bearer <access token>
Content-Type: application/x-www-form-urlencoded

topic=https%3A%2F%2Fexample.com%2Fbooks%2F1&data=%7B%22status%22%3A%22checked+out%22%7D
```

The hub fans the update out to every subscriber whose matchers hit one of the publication's topics, then returns the event ID it assigned.

## Mercure publish form fields

| Field     | Required | Description                                                                                                                                                                              |
|-----------|----------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `topic`   | Yes      | Identifier of the updated topic. **MAY** appear more than once: the first occurrence is the canonical topic, any others are [alternate topics](topics-and-matchers.md#alternate-topics). |
| `data`    | No       | Payload of the update. Anything you want: JSON, HTML, JSON Patch, plain text.                                                                                                            |
| `private` | No       | If present, the update is private. The hub delivers it only to subscribers authorized for the topic.                                                                                     |
| `id`      | No       | Custom event ID. Must not start with `#` or equal the reserved value `earliest`. The hub assigns one if you don't.                                                                       |
| `type`    | No       | Custom SSE `event` type. Defaults to `message`. `mercure` is reserved for hub-generated events and is rejected with a `400`.                                                             |
| `retry`   | No       | Reconnection time hint, in milliseconds.                                                                                                                                                 |

The body is `application/x-www-form-urlencoded`: every field is URL-encoded.

The hub treats `data` as opaque bytes, so you can push any format the subscriber
understands: JSON, HTML, plain text, JSON Patch, [raw binary](#publishing-binary-data), or an
event envelope such as [CloudEvents](https://cloudevents.io/) or
[ActivityStreams 2.0](https://www.w3.org/TR/activitystreams-core/). Wrapping the
payload in an envelope is a publisher/subscriber convention; the hub neither
requires nor inspects it. See [Update payloads](update-payloads.md) for how to
pick one.

## Publishing binary data

When the hub runs with the [`events_query` directive](../deployment/configuration.md#mercure-directives) (experimental), the publish endpoint also accepts `multipart/form-data` bodies. The fields are the same, but the `data` part carries raw bytes, and its `Content-Type` header, if set, declares the media type of the payload (invalid media types are rejected with a `400`). Without the directive, multipart publications are rejected with a `415`.

```console
# Publishing binary data
curl -X POST https://hub.example.com/.well-known/mercure \
  -H "Authorization: Bearer $JWT" \
  -F 'topic=https://example.com/books/1/cover' \
  -F 'data=@cover.png;type=image/png'
```

Delivery depends on the subscription encoding:

- [HTTP Events Query subscribers](subscribing.md#subscribing-with-http-events-query) (`multipart/mixed`) receive the bytes verbatim, with the declared media type in the part's `Content-Type` header.
- Server-Sent Events subscribers receive the `data` field base64-encoded. This happens for every multipart-published update, even when the payload is plain text: SSE has no per-event metadata slot, so only a rule fixed at publication time lets subscribers decode deterministically.

Multipart publishing is also the way to get byte-exact round-tripping of text payloads (encrypted blobs, signed documents): the SSE serialization normalizes CR and CRLF to LF, while base64 (over SSE) and multipart parts both preserve the exact bytes.

## Mercure publish examples

### Publishing to Mercure with `curl`

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

## Canonical and alternate topics

A publish request usually carries a single `topic`: pick one canonical topic for a resource (its URL is the natural choice) and use it consistently on both the publish and subscribe sides.

The `topic` field **MAY** be repeated. The first occurrence is the canonical topic; any others are alternate topics, and the hub dispatches the update to subscribers matching either the canonical topic or any alternate — see [Alternate topics](topics-and-matchers.md#alternate-topics). This lets one publish serve several differently-scoped private audiences at once, instead of one publish per audience; see [Authorization](authorization.md#per-user-authorization-on-shared-resources) for the pattern and its confidentiality rule.

## Public vs. Private updates

Without the `private` field, an update is **public**: the hub sends it to every subscriber whose matchers hit, regardless of whether they presented a token.

With `private=on` (the value can be anything; `on` is the convention), the update is **private**: a subscriber receives it only if its token grants `subscribe` on at least one of the update's topics.

```console
# Public, anyone subscribed to this topic gets it
curl -X POST $HUB -H "Authorization: Bearer $JWT" \
  -d 'topic=https://example.com/news/latest' \
  -d 'data=...'

# Private, only authorized subscribers get it
curl -X POST $HUB -H "Authorization: Bearer $JWT" \
  -d 'topic=https://example.com/users/42/inbox' \
  -d 'data=...' \
  -d 'private=on'
```

If you want updates on a topic to be visible only to authorized subscribers, **mark them private**. The hub does not infer privacy from the topic URL.

## Authorization

The publisher's access token must carry an `authorization_details` entry whose `actions` include `publish` and whose `topics` cover every topic of the publication — the canonical topic and any alternates. Otherwise the hub returns `403 insufficient_scope` (or `401` when no token is presented).

```jsonc
// Authorization
{
  "authorization_details": [
    {
      "type": "https://mercure.rocks/authorization-detail",
      "actions": ["publish"],
      "topics": [
        {
          "match": "https://example.com/books/:id",
          "match_type": "urlpattern",
        },
      ],
    },
  ],
}
```

A grant of `[{ "match": "*" }]` can publish to anything. See [Authorization](authorization.md#publishers) for details.

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

## When to publish Mercure updates from your application

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

## Next steps for Mercure publishing

- [Authorization](authorization.md): minting JWTs that pass validation.
- [Active subscriptions](active-subscriptions.md): knowing who's connected.
- [Reconnection and history](reconnection-and-history.md): making sure subscribers don't miss updates.
