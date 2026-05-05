---
title: "Hotwire Turbo Streams over Mercure"
description: "Push HTML fragments to the browser using Hotwire Turbo Streams and Mercure with a few lines of JavaScript glue."
---

# Hotwire / Turbo Streams

[Hotwire](https://hotwire.dev) sends HTML over the wire instead of JSON. [Turbo Streams](https://turbo.hotwire.dev/handbook/streams) let the server push HTML fragments that the browser splices into the page — append a row, replace a region, remove a node.

Mercure is a clean transport for Turbo Streams. No extra dependency on the server; on the client, three lines of glue.

## Subscribe to Hotwire Turbo Streams via Mercure

```javascript
// Subscribe to Hotwire Turbo Streams via Mercure
import { connectStreamSource } from "@hotwired/turbo";

const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append("match", "https://example.com/posts/42/comments");

const es = new EventSource(url);
connectStreamSource(es);
```

Turbo treats every SSE message as a Turbo Stream and applies it. The `data` of each message is HTML in the Turbo Stream format.

## Publish Turbo Streams to Mercure

The server publishes Turbo Stream HTML on the matching topic:

```html
<!-- Publish Turbo Streams to Mercure -->
<turbo-stream action="append" target="comments">
  <template>
    <li id="comment_99">Great post!</li>
  </template>
</turbo-stream>
```

```console
# Publish Turbo Streams to Mercure
curl -X POST https://hub.example.com/.well-known/mercure \
  -H "Authorization: Bearer $JWT" \
  -d 'topic=https://example.com/posts/42/comments' \
  --data-urlencode 'data=<turbo-stream action="append" target="comments"><template><li id="comment_99">Great post!</li></template></turbo-stream>'
```

In Rails:

```ruby
# In a controller after the comment is created
Mercure.publish(
  topic: post_comments_url(@post),
  data: render_to_string(partial: "comments/turbo_append", locals: { comment: @comment }),
)
```

In Symfony with the [Mercure component](https://symfony.com/doc/current/mercure.html):

```php
// Publish Turbo Streams to Mercure
$update = new Update(
    $this->generateUrl('comments', ['post' => $post->getId()]),
    $this->renderView('comments/_append.html.twig', ['comment' => $comment]),
);
$hub->publish($update);
```

## Disconnecting a Turbo Stream Source from Mercure

```javascript
// Disconnecting a Turbo Stream Source from Mercure
import { disconnectStreamSource } from "@hotwired/turbo";

es.close();
disconnectStreamSource(es);
```

Always disconnect when the page (or component) using the stream goes away.

## A Stimulus Controller for Mercure Turbo Streams

Wire the stream into a `<div>` and let Stimulus manage its lifecycle:

```javascript
// turbo_stream_controller.js
import { Controller } from "@hotwired/stimulus";
import { connectStreamSource, disconnectStreamSource } from "@hotwired/turbo";

export default class extends Controller {
  static values = { url: String };

  connect() {
    this.es = new EventSource(this.urlValue, { withCredentials: true });
    connectStreamSource(this.es);
  }

  disconnect() {
    this.es.close();
    disconnectStreamSource(this.es);
  }
}
```

```html
<!-- A Stimulus Controller for Mercure Turbo Streams -->
<div
  data-controller="turbo-stream"
  data-turbo-stream-url-value="https://hub.example.com/.well-known/mercure?match=https%3A%2F%2Fexample.com%2Fposts%2F42%2Fcomments"
>
  <ul id="comments">
    <!-- server-rendered initial state -->
  </ul>
</div>
```

The stream goes live on `connect` (when the element enters the DOM) and shuts down on `disconnect`. Turbo Drive navigations don't drop the stream in unexpected ways.

## Private Turbo Streams Over Mercure

For per-user or per-team streams (a kanban board only the team's members can see), authorize via cookie:

```jsonc
// Private Turbo Streams over Mercure
{
  "mercure": {
    "subscribe": [
      { "match": "https://example.com/teams/acme/board" },
      { "match": "https://example.com/users/42/notifications" }
    ]
  }
}
```

Publish the Turbo Stream as a private update (`private=on`). Only authorized subscribers receive it.

The cookie should be set during the page render (not in JavaScript) so that `EventSource(url, { withCredentials: true })` already has it. See [Authorization](../concepts/authorization.md#cookies-in-detail).

## Many Streams, One Connection

A page often watches several streams: comments, presence, notifications, a sidebar counter. Use `match*` parameters on a single connection rather than spinning up four `EventSource`s:

```javascript
// Many streams, one connection
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append("match", "https://example.com/posts/42/comments");
url.searchParams.append("match", "https://example.com/posts/42/votes");
url.searchParams.append("matchURLPattern", "https://example.com/users/:id/notifications");
```

Turbo applies whichever stream is in the `data`; the `target` attribute on each `<turbo-stream>` element decides where it lands.

## Hotwire and Mercure Rendering Performance

Turbo Stream HTML is just bytes — no different from JSON for the hub. The cost is on the rendering side: every connected user re-runs `morphdom` (or whichever DOM patcher Turbo uses) on each message. Avoid publishing 100 streams a second to a page; coalesce on the server, or fall back to a JSON delta you render yourself.

## Hotwire Native (iOS / Android)

The same Mercure topic works for Hotwire Native apps — the bridge ships an SSE consumer. Use the platform's `EventSource`-equivalent (or [`fetch-event-source`](https://github.com/Azure/fetch-event-source)) and feed bytes into the Turbo Native stream renderer.

## Next Steps for Hotwire Over Mercure

- [Subscribing](../concepts/subscribing.md) — `EventSource` details.
- [Authorization](../concepts/authorization.md) — cookies for browsers.
- [Collaborative editing](collaborative-editing.md) — for editing scenarios where Turbo Streams aren't enough.
