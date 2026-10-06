---
title: "Publishing real-time updates to a Mercure hub"
description: "Send POST requests to publish public and private updates to a Mercure hub and authorize publishers with an OAuth 2.0 access token."
---

# Publish Mercure updates

To publish over HTTP, send a `POST` to the hub with a form-encoded body. An embedded hub can also accept updates directly through a library API; the protocol does not require an HTTP publication endpoint. See [Embedded publishing](#embedded-publishing-no-external-hub).

```http
POST /.well-known/mercure
Host: hub.example.com
Authorization: Bearer <access token>
Content-Type: application/x-www-form-urlencoded

topic=https%3A%2F%2Fexample.com%2Fbooks%2F1&data=%7B%22status%22%3A%22checked+out%22%7D
```

The hub authorizes the publication, stores and dispatches it through the configured transport, then returns an event ID. Private updates reach only authorized subscribers.

## Mercure publish form fields

| Field     | Required | Description                                                                                                                                                                              |
| --------- | -------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `topic`   | Yes      | Identifier of the updated topic. **MAY** appear more than once: the first occurrence is the canonical topic, any others are [alternate topics](topics-and-matchers.md#alternate-topics). |
| `data`    | No       | Payload of the update. Anything you want: JSON, HTML, JSON Patch, plain text.                                                                                                            |
| `private` | No       | If present, the update is private. The hub delivers it only to subscribers authorized for the topic.                                                                                     |
| `id`      | No       | Custom event ID. Must not start with `#` or equal the reserved value `earliest`. The hub assigns one if you don't.                                                                       |
| `type`    | No       | Custom SSE `event` type. Defaults to `message`. `mercure` is reserved for hub-generated events and is rejected with a `400`.                                                             |
| `retry`   | No       | Reconnection time hint, in milliseconds.                                                                                                                                                 |

The body is `application/x-www-form-urlencoded`: URL-encode every field. This hub accepts at most 1,000 topic fields and limits request bodies to 1 MiB by default (`max_request_body_size`).

The hub treats `data` as opaque bytes, so you can push any format the subscriber
understands: JSON, HTML, plain text, JSON Patch, base64-encoded binary, or an
event envelope such as [CloudEvents](https://cloudevents.io/) or
[ActivityStreams 2.0](https://www.w3.org/TR/activitystreams-core/). Wrapping the
payload in an envelope is a publisher/subscriber convention; the hub neither
requires nor inspects it. See [Update payloads](update-payloads.md) for how to
pick one.

## Mercure publish examples

### Publishing to Mercure with `curl`

```console
curl -X POST https://hub.example.com/.well-known/mercure \
  -H "Authorization: Bearer $JWT" \
  --data-urlencode 'topic=https://example.com/books/1' \
  --data-urlencode 'data={"status": "checked out"}'
```

### Publishing to Mercure from Node.js

```javascript
const response = await fetch("https://hub.example.com/.well-known/mercure", {
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
if (!response.ok) throw new Error(`Publish failed: ${response.status}`);
```

### Publishing to Mercure from Python

```python
import requests

response = requests.post(
    "https://hub.example.com/.well-known/mercure",
    headers={"Authorization": f"Bearer {jwt}"},
    data={"topic": "https://example.com/books/1", "data": '{"status": "checked out"}'},
    timeout=10,
)
response.raise_for_status()
```

### Publishing to Mercure from PHP with Symfony

The [Symfony Mercure component](https://symfony.com/doc/current/mercure.html) wraps the protocol:

```php
use Symfony\Component\Mercure\HubInterface;
use Symfony\Component\Mercure\Update;

function publishBookUpdate(HubInterface $hub): void {
    $hub->publish(new Update(
        'https://example.com/books/1',
        json_encode(['status' => 'checked out']),
    ));
}
```

Inject `HubInterface` in a Symfony controller or service. The [Symfony Mercure guide](https://symfony.com/doc/current/mercure.html) covers hub configuration, token generation, and private updates. The component also works outside the Symfony framework.

### Publishing to Mercure from Laravel

Laravel's native Mercure broadcasting driver publishes your broadcast events through the hub. Keep using `ShouldBroadcast`, private channels, and Laravel Echo. Follow the [Laravel broadcasting guide](../use-cases/laravel-broadcasting.md) for a complete example.

## Canonical and alternate topics

A publish request usually carries a single `topic`: pick one canonical topic for a resource (its URL is the natural choice) and use it consistently on both the publish and subscribe sides.

The `topic` field **MAY** be repeated. The first occurrence is the canonical topic; any others are alternate topics, and the hub dispatches the update to subscribers matching either the canonical topic or any alternate; see [Alternate topics](topics-and-matchers.md#alternate-topics). This lets one publish serve several differently-scoped private audiences at once, instead of one publish per audience; see [Authorization](authorization.md#per-user-authorization-on-shared-resources) for the pattern and its confidentiality rule.

## Public vs. Private updates

Without the `private` field, an update is **public**: the hub sends it to every subscriber whose matchers hit, regardless of whether they presented a token.

With `private=on` (the value can be anything; `on` is the convention), the update is **private**: a subscriber receives it only if its token grants `subscribe` on at least one of the update's topics.

```console
curl -X POST "$HUB" -H "Authorization: Bearer $JWT" \
  --data-urlencode 'topic=https://example.com/news/latest' \
  --data-urlencode 'data=...'

# Private, only authorized subscribers get it
curl -X POST "$HUB" -H "Authorization: Bearer $JWT" \
  --data-urlencode 'topic=https://example.com/users/42/inbox' \
  --data-urlencode 'data=...' \
  --data-urlencode 'private=on'
```

If you want updates on a topic to be visible only to authorized subscribers, **mark them private**. The hub does not infer privacy from the topic URL.

## Authorization

The publisher's access token must carry an `authorization_details` entry whose `actions` include `publish` and whose `topics` cover every topic of the publication; the canonical topic and any alternates. Otherwise the hub returns `403 insufficient_scope` (or `401` when no token is presented).

```json
{
  "authorization_details": [
    {
      "type": "https://mercure.rocks/authorization-detail",
      "actions": ["publish"],
      "topics": [
        {
          "match": "https://example.com/books/:id",
          "match_type": "urlpattern"
        }
      ]
    }
  ]
}
```

A grant of `[{ "match": "*" }]` can publish to anything. See [Authorization](authorization.md#publishers) for details.

## What the hub returns

```http
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
function updateBook(id, data):
    db.update(id, data)
    hub.publish(
        topic="https://example.com/books/" + id,
        data=json(data),
    )
```

For retryable publishing, use a transactional outbox: commit the application change and an outgoing event in the same database transaction, then have a worker publish it and retry failures. Retries can create duplicates, so make consumers idempotent.

## Embedded publishing (no external hub)

The [Mercure specification](../../spec/mercure.md#publication) makes the HTTP publication endpoint optional. An application can embed the [Go library](https://pkg.go.dev/github.com/dunglas/mercure) and publish directly to the hub in the same process, while subscribers still use the Mercure protocol.

[FrankenPHP embeds Mercure](https://frankenphp.dev/docs/mercure/) and exposes `mercure_publish()` to PHP:

```php
$eventId = mercure_publish(
    'https://example.com/books/1',
    json_encode(['status' => 'checked out'], JSON_THROW_ON_ERROR),
);
```

Enable the built-in hub in FrankenPHP's Caddyfile first. This function publishes directly, without an HTTP request or publisher JWT. Protect the application action that calls it; subscriber authorization still applies to private updates.

## Next steps

- [Authorization](authorization.md): minting JWTs that pass validation.
- [Active subscriptions](active-subscriptions.md): knowing who's connected.
- [Reconnection and history](reconnection-and-history.md): making sure subscribers don't miss updates.
