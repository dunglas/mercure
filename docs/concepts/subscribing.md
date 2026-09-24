---
title: "Subscribing to Mercure updates with server-sent events"
description: "Subscribe to Mercure from browsers, PHP with Symfony HttpClient, Laravel Echo, Node.js, Go, and Python. Learn SSE connection limits and topic subscriptions."
---

# Subscribe to Mercure updates

A subscription is an HTTP `GET` request to the hub's well-known URL that the hub keeps open and writes [Server-Sent Events](https://html.spec.whatwg.org/multipage/server-sent-events.html) into.

```http
GET /.well-known/mercure?match=https://example.com/books/1 HTTP/2
Host: hub.example.com
Accept: text/event-stream
```

Pick the matchers you want with [`match*` query parameters](topics-and-matchers.md). The rest of this page covers client code.

## Subscribing from a browser with EventSource

`EventSource` is built into every modern browser:

```javascript
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append("match", "https://example.com/books/1");
url.searchParams.append("match_urlpattern", "https://example.com/users/:id");

const es = new EventSource(url);
es.onmessage = (event) => {
  const data = JSON.parse(event.data);
  // ...
};
es.onerror = () => {
  // The browser auto-reconnects; this fires on each retry.
};
```

A few things to know:

- Browsers typically allow **6 HTTP/1.1 connections per origin**, shared across tabs. With HTTP/2, **100 concurrent streams is a common default**, negotiated between client and server. These are streams sharing a connection, not 100 separate TCP connections. See [MDN's SSE connection limits](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#receiving_events_from_the_server). HTTP/3 also multiplexes streams, with its own negotiated limits.
- A single `EventSource` connection can carry up to 100 matchers in this hub by passing more `match*` parameters.
- `EventSource` does not let you set `Authorization` headers. For private subscriptions, use the [`__Secure-mercure_access_token` cookie](authorization.md#cookies-in-detail), or consume the stream with `fetch()` and an `Authorization` header when a cookie can't work (per-tab tokens, cross-domain hub).

### Subscribing with Laravel Echo

Laravel Echo has native Mercure support. Use Echo's channel, private-channel, and presence APIs with Mercure as the broadcaster. See [Laravel broadcasting with Mercure](../use-cases/laravel-broadcasting.md) for server configuration and browser examples.

### `fetch-event-source` for advanced cases

For custom headers, retry handling, or `QUERY` requests, use [Microsoft's `fetch-event-source`](https://github.com/Azure/fetch-event-source):

```javascript
import { fetchEventSource } from "@microsoft/fetch-event-source";

await fetchEventSource(url, {
  headers: { Authorization: `Bearer ${jwt}` },
  onmessage: (event) => {
    console.log(event.data);
  },
  onerror: (err) => {
    console.error(err);
  },
});
```

`fetch-event-source` uses browser APIs, including `document` and `window`. For Node.js, use the `eventsource` package shown below. For mobile runtimes, choose an SSE client that supports that runtime.

## Subscribing with the QUERY method

Long topic strings can exceed URL limits imposed by browsers or proxies. `QUERY` moves the matchers into the body; it does not remove the hub's limit of 100 matchers per subscription.

To avoid this, the hub also accepts the [`QUERY` HTTP method](https://www.rfc-editor.org/rfc/rfc10008.html) on the subscription URL. `QUERY` is safe and idempotent like `GET`, but carries the topic matcher parameters in the request body instead of the query string. The body **must** be encoded as `application/x-www-form-urlencoded`, exactly as the query string would be:

```http
QUERY /.well-known/mercure HTTP/2
Host: hub.example.com
Accept: text/event-stream
Content-Type: application/x-www-form-urlencoded

match=https://example.com/books/1&match_urlpattern=https://example.com/users/:id
```

`EventSource` only issues `GET` requests, so it can't use `QUERY`. Reach for a `fetch`-based client:

```javascript
import { fetchEventSource } from "@microsoft/fetch-event-source";

const body = new URLSearchParams();
body.append("match", "https://example.com/books/1");
body.append("match_urlpattern", "https://example.com/users/:id");

await fetchEventSource("https://hub.example.com/.well-known/mercure", {
  method: "QUERY",
  headers: { "Content-Type": "application/x-www-form-urlencoded" },
  body,
  onmessage: (event) => {
    console.log(event.data);
  },
});
```

Everything else stays the same: the matcher names and values follow the [same rules](topics-and-matchers.md) as query parameters, and the hub streams the same Server-Sent Events. Stick with `GET` unless your topic list is large enough to risk the URL limit.

## Closing the Mercure EventSource connection

`EventSource` keeps a subscription stream open until you close it or the page unloads. In single-page apps the connection survives component unmounts and route changes if you don't tear it down explicitly:

```javascript
useEffect(() => {
  const es = new EventSource(url);
  es.onmessage = handler;
  return () => es.close();
}, [url]);
```

Without cleanup, unused subscriptions keep consuming browser and hub resources.

## Server-side subscribers

Any HTTP client that exposes a streaming response works. A few examples:

### Subscribing to Mercure from PHP with Symfony HttpClient

Symfony's [`EventSourceHttpClient`](https://symfony.com/doc/current/http_client.html#consuming-server-sent-events) parses SSE streams for you. It works in any PHP application; the Symfony framework is not required.

Install it with `composer require symfony/http-client`, then save this as `subscribe.php` and run `php subscribe.php`. Set `MERCURE_URL` and a subscriber token in `MERCURE_JWT`:

```php
<?php

require __DIR__.'/vendor/autoload.php';

use Symfony\Component\HttpClient\Chunk\ServerSentEvent;
use Symfony\Component\HttpClient\EventSourceHttpClient;

$client = new EventSourceHttpClient();
$source = $client->connect(getenv('MERCURE_URL'), [
    'query' => ['match' => 'https://example.com/books/1'],
    'auth_bearer' => getenv('MERCURE_JWT'),
]);

try {
    foreach ($client->stream($source) as $chunk) {
        if ($chunk->isTimeout()) {
            continue;
        }
        if ($chunk instanceof ServerSentEvent) {
            echo $chunk->getData(), PHP_EOL;
        }
    }
} finally {
    $source->cancel();
}
```

Use `getArrayData()` for JSON payloads, `getType()` for named events, and `getId()` to save a replay cursor. Omit `auth_bearer` only when the hub allows anonymous subscriptions and you need public updates.

### Subscribing to Mercure from Node.js

Install the [`eventsource` package](https://github.com/EventSource/eventsource), then:

```javascript
import { EventSource } from "eventsource";

const es = new EventSource(
  "https://hub.example.com/.well-known/mercure?match=topic",
);
es.onmessage = (event) => console.log(event.data);
```

This example requires anonymous subscriptions. Use the package's custom `fetch` option to attach an `Authorization` header on a protected hub.

### Subscribing to Mercure from Go

Install `github.com/r3labs/sse/v2`. A complete subscriber:

```go
package main

import (
    "fmt"
    "log"

    "github.com/r3labs/sse/v2"
)

func main() {
    client := sse.NewClient("https://hub.example.com/.well-known/mercure?match=topic")
    if err := client.SubscribeRaw(func(msg *sse.Event) {
        fmt.Println(string(msg.Data))
    }); err != nil {
        log.Fatal(err)
    }
}
```

### Subscribing to Mercure from Python

Install `sseclient` for this API (the similarly named `sseclient-py` package has a different constructor). The example requires anonymous subscriptions.

```python
from sseclient import SSEClient

for event in SSEClient("https://hub.example.com/.well-known/mercure?match=topic"):
    print(event.data)
```

[Awesome Mercure](../ecosystem/awesome.md) lists more libraries.

## What the hub sends

Each event is a standard SSE message:

```text
id: urn:uuid:e1ee88e2-532a-4d6f-ba70-f0f8bd584022
event: message
data: {"status": "checked out"}

```

Fields:

- `id`: a unique identifier the hub assigns to every update. Clients send it back in `Last-Event-ID` to resume after a disconnect. See [Reconnection and history](reconnection-and-history.md).
- `event`: the `type` field from the publish request, if any. Defaults to `message`. `EventSource` triggers `addEventListener("<type>", ...)` for non-default types.
- `data`: whatever the publisher sent in `data`. Mercure does not interpret it; it's bytes you decided on (JSON, HTML, JSON Patch, plain text...).

## Discovering the Mercure hub via link header

A resource can advertise its hub in a `Link` header. The following exchange shows a discovery response:

```http
GET /books/1
Host: example.com

200 OK
Content-Type: application/ld+json
Link: <https://hub.example.com/.well-known/mercure>; rel="mercure"
```

Subscribers that fetch the resource first can read the header to find the hub:

```javascript
const res = await fetch("https://example.com/books/1");
const link = res.headers.get("Link");
const hub = link?.match(/<([^>]+)>;\s*rel="?mercure"?/)?.[1];
if (!hub) throw new Error("No Mercure link in the response");

const url = new URL(hub, res.url);
url.searchParams.append(
  "match",
  new URL(res.headers.get("Content-Location") ?? res.url, res.url).href,
);
new EventSource(url);
```

The regular expression handles the simple header shown above. For arbitrary `Link` headers, use a Web Linking parser. Cross-origin discovery responses must expose `Link` and `Content-Location` through `Access-Control-Expose-Headers`. See [Discovery](discovery.md).

## Mercure SSE heartbeats

The hub sends an SSE comment every `heartbeat` seconds (default `40s`). Heartbeats keep idle connections alive through proxies that close them after silence and let clients detect dead connections faster.

If you set `heartbeat 0s` to disable them, make sure nothing on the network path does idle-timeout TCP. Most CDNs and reverse proxies do.

## Mercure subscriber connection limits

| Limit                                      | Where                                                                                  |
| ------------------------------------------ | -------------------------------------------------------------------------------------- |
| Concurrent HTTP/2 streams per connection   | Common default: 100; negotiated by client and server                                   |
| Concurrent HTTP/1.1 connections per origin | Typically 6; browser-dependent                                                         |
| Concurrent connections to the hub          | Hardware-bound on OSS; see [Cloud and Enterprise plans](https://mercure.rocks/pricing) |

A single connection accepts up to 100 matchers in this hub: pass more `match*` parameters rather than opening more `EventSource` instances.

## Next steps

- [Topics and matchers](topics-and-matchers.md): choosing the right matcher type.
- [Reconnection and history](reconnection-and-history.md): surviving disconnects without losing events.
- [Active subscriptions](active-subscriptions.md): presence and the subscription API.
