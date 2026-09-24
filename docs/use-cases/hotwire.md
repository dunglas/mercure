---
title: "Hotwire Turbo Streams over Mercure"
description: "Push HTML fragments to the browser using Hotwire Turbo Streams and Mercure with a few lines of JavaScript glue."
---

# Hotwire Turbo Streams with Mercure

[Hotwire](https://hotwire.dev) sends HTML over the wire instead of JSON. [Turbo Streams](https://turbo.hotwire.dev/handbook/streams) let the server push HTML fragments that the browser splices into the page: append a row, replace a region, remove a node.

Mercure can deliver Turbo Stream HTML through an `EventSource` connected with Turbo's `connectStreamSource` API.

## Subscribe to Hotwire Turbo Streams via Mercure

```javascript
import { connectStreamSource } from "@hotwired/turbo";

const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append("match", "https://example.com/posts/42/comments");

const es = new EventSource(url);
connectStreamSource(es);
```

Turbo consumes default SSE `message` events. Publish Turbo Stream HTML in `data` and leave the Mercure `type` field unset.

## Publish Turbo Streams to Mercure

The server publishes Turbo Stream HTML on the matching topic:

```html
<turbo-stream action="append" target="comments">
  <template>
    <li id="comment_99">Great post!</li>
  </template>
</turbo-stream>
```

```console
curl -X POST https://hub.example.com/.well-known/mercure \
  -H "Authorization: Bearer $JWT" \
  --data-urlencode 'topic=https://example.com/posts/42/comments' \
  --data-urlencode 'data=<turbo-stream action="append" target="comments"><template><li id="comment_99">Great post!</li></template></turbo-stream>'
```

In Rails, using Ruby's standard HTTP client:

```ruby
require "net/http"

uri = URI(ENV.fetch("MERCURE_URL"))
request = Net::HTTP::Post.new(uri)
request["Authorization"] = "Bearer #{ENV.fetch('MERCURE_PUBLISHER_JWT')}"
request.set_form_data(
  "topic" => post_comments_url(@post),
  "data" => render_to_string(partial: "comments/turbo_append", locals: { comment: @comment }),
)
response = Net::HTTP.start(uri.host, uri.port, use_ssl: uri.scheme == "https") do |http|
  http.request(request)
end
raise "Mercure publish failed: #{response.code}" unless response.is_a?(Net::HTTPSuccess)
```

In Symfony with the [Mercure component](https://symfony.com/doc/current/mercure.html):

```php
use Symfony\Component\Mercure\Update;
use Symfony\Component\Routing\Generator\UrlGeneratorInterface;

$update = new Update(
    $this->generateUrl('comments', ['post' => $post->getId()], UrlGeneratorInterface::ABSOLUTE_URL),
    $this->renderView('comments/_append.html.twig', ['comment' => $comment]),
);
$hub->publish($update);
```

## Disconnecting a Turbo Stream source from Mercure

```javascript
import { disconnectStreamSource } from "@hotwired/turbo";

es.close();
disconnectStreamSource(es);
```

Always disconnect when the page (or component) using the stream goes away.

## A Stimulus controller for Mercure Turbo streams

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
<div
  data-controller="turbo-stream"
  data-turbo-stream-url-value="https://hub.example.com/.well-known/mercure?match=https%3A%2F%2Fexample.com%2Fposts%2F42%2Fcomments"
>
  <ul id="comments">
    <!-- server-rendered initial state -->
  </ul>
</div>
```

The stream goes live on `connect` (when the element enters the DOM) and shuts down on `disconnect`. The controller closes its connection during Turbo Drive navigation and opens a new one when reconnected.

## Private Turbo Streams over Mercure

For per-user or per-team streams (a kanban board only the team's members can see), authorize via cookie:

```json
{
  "iss": "https://example.com",
  "aud": "https://hub.example.com/.well-known/mercure",
  "exp": 4102444800,
  "authorization_details": [
    {
      "type": "https://mercure.rocks/authorization-detail",
      "actions": ["subscribe"],
      "topics": [
        { "match": "https://example.com/teams/acme/board" },
        { "match": "https://example.com/users/42/notifications" }
      ]
    }
  ]
}
```

Publish the Turbo Stream as a private update (`private=on`). Only authorized subscribers receive it.

The cookie should be set during the page render (not in JavaScript) so that `EventSource(url, { withCredentials: true })` already has it. See [Authorization](../concepts/authorization.md#cookies-in-detail).

## Many streams, one connection

A page can watch several HTML streams, such as comments and vote counts. Use `match*` parameters on a single connection rather than spinning up four `EventSource`s:

```javascript
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append("match", "https://example.com/posts/42/comments");
url.searchParams.append("match", "https://example.com/posts/42/votes");
url.searchParams.append(
  "match_urlpattern",
  "https://example.com/users/:id/notifications",
);
const es = new EventSource(url, { withCredentials: true });
connectStreamSource(es);
```

Turbo applies whichever stream is in the `data`; the `target` attribute on each `<turbo-stream>` element decides where it lands.

## Hotwire and Mercure rendering performance

Rendering cost depends on the Turbo action and DOM size. Batch frequent changes and measure rendering time. Escape untrusted content in server templates before publishing HTML.

## Hotwire native (iOS / Android)

In a Hotwire Native web view, connect the page's `EventSource` to Turbo as you would in a browser. Test cookies, backgrounding, and reconnection on each platform; this does not provide native background push notifications.

## Next steps

- [Subscribing](../concepts/subscribing.md): `EventSource` details.
- [Authorization](../concepts/authorization.md): cookies for browsers.
- [Collaborative editing](collaborative-editing.md): for editing scenarios where Turbo Streams aren't enough.
