---
title: "Subscribing to Mercure updates with server-sent events"
description: "Open SSE subscriptions to a Mercure hub from browsers, Node.js, Go, Python, and other clients with EventSource and fetch-event-source."
---

# Subscribing

A subscription is an HTTP `GET` request to the hub's well-known URL that the hub keeps open and writes [Server-Sent Events](https://html.spec.whatwg.org/multipage/server-sent-events.html) into.

```http
# Subscribing
GET /.well-known/mercure?match=https://example.com/books/1 HTTP/2
Host: hub.example.com
Accept: text/event-stream
```

Pick the matchers you want with [`match*` query parameters](topics-and-matchers.md). The rest of this page covers the client-side.

## Subscribing from a browser with EventSource

`EventSource` is built into every modern browser:

```javascript
// Subscribing from a Browser with EventSource
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

- Browsers cap concurrent HTTP/1.1 requests per origin at 6. With HTTP/2 (the default everywhere on HTTPS) the cap is 100 streams negotiated with the server. **Use HTTP/2**: your hub already speaks it.
- A single `EventSource` connection can carry as many topic subscriptions as you want by passing more `match*` parameters.
- `EventSource` does not let you set `Authorization` headers. For private subscriptions, use the [`mercure_access_token` cookie](authorization.md#cookies-in-detail), or consume the stream with `fetch()` and an `Authorization` header when a cookie can't work (per-tab tokens, cross-domain hub).

### `fetch-event-source` for advanced cases

For finer-grained error handling, custom headers, or non-`GET` requests, use [Microsoft's `fetch-event-source`](https://github.com/Azure/fetch-event-source):

```javascript
// fetch-event-source for advanced cases
import { fetchEventSource } from "@microsoft/fetch-event-source";

await fetchEventSource(url, {
  headers: { Authorization: `Bearer ${jwt}` },
  onmessage: (event) => /* ... */,
  onerror: (err) => /* ... */,
});
```

This is the right choice for React Native, server-side runtimes, and any case where the native `EventSource` is too constrained.

## Subscribing with the QUERY method

Each `match*` parameter adds to the subscription URL. A subscriber watching hundreds of topics can hit the URL length limits that browsers, proxies, and CDNs enforce (commonly 4 to 8 KB), and the request fails before it reaches the hub.

To avoid this, the hub also accepts the [`QUERY` HTTP method](https://www.rfc-editor.org/rfc/rfc10008.html) on the subscription URL. `QUERY` is safe and idempotent like `GET`, but carries the topic matcher parameters in the request body instead of the query string. The body **must** be encoded as `application/x-www-form-urlencoded`, exactly as the query string would be:

```http
# Subscribing with the QUERY method
QUERY /.well-known/mercure HTTP/2
Host: hub.example.com
Accept: text/event-stream
Content-Type: application/x-www-form-urlencoded

match=https://example.com/books/1&match_urlpattern=https://example.com/users/:id
```

`EventSource` only issues `GET` requests, so it can't use `QUERY`. Reach for a `fetch`-based client:

```javascript
// Subscribing with the QUERY method
import { fetchEventSource } from "@microsoft/fetch-event-source";

const body = new URLSearchParams();
body.append("match", "https://example.com/books/1");
body.append("match_urlpattern", "https://example.com/users/:id");

await fetchEventSource("https://hub.example.com/.well-known/mercure", {
  method: "QUERY",
  headers: { "Content-Type": "application/x-www-form-urlencoded" },
  body,
  onmessage: (event) => /* ... */,
});
```

Everything else stays the same: the matcher names and values follow the [same rules](topics-and-matchers.md) as query parameters, and the hub streams the same Server-Sent Events. Stick with `GET` unless your topic list is large enough to risk the URL limit.

## Closing the Mercure EventSource connection

`EventSource` keeps the TCP connection open until you close it. In single-page apps the connection survives component unmounts and route changes if you don't tear it down explicitly:

```javascript
// Closing the Mercure EventSource Connection
useEffect(() => {
  const es = new EventSource(url);
  es.onmessage = handler;
  return () => es.close();
}, [url]);
```

Skipping this is the most common cause of "the hub keeps a slot for me even though I navigated away." The hub sees the connection as live and counts it against any per-IP or per-token limits.

## Server-side subscribers

Any HTTP client that exposes a streaming response works. A few examples:

### Subscribing to Mercure from Node.js

```javascript
// Subscribing to Mercure from Node.js
import { fetchEventSource } from "@microsoft/fetch-event-source";

await fetchEventSource(
  "https://hub.example.com/.well-known/mercure?match=topic",
  {
    onmessage: (e) => console.log(e.data),
  },
);
```

### Subscribing to Mercure from Go

```go
// Subscribing to Mercure from Go
import "github.com/r3labs/sse/v2"

client := sse.NewClient("https://hub.example.com/.well-known/mercure?match=topic")
client.Subscribe("messages", func(msg *sse.Event) {
    fmt.Println(string(msg.Data))
})
```

### Subscribing to Mercure from Python

```python
# Subscribing to Mercure from Python
from sseclient import SSEClient

for event in SSEClient("https://hub.example.com/.well-known/mercure?match=topic"):
    print(event.data)
```

[Awesome Mercure](../ecosystem/awesome.md) lists more libraries.

## What the hub sends

Each event is a standard SSE message:

```text
# What the hub sends
id: urn:uuid:e1ee88e2-532a-4d6f-ba70-f0f8bd584022
event: message
data: {"status": "checked out"}

```

Fields:

- `id`: a unique identifier the hub assigns to every update. Clients send it back in `Last-Event-ID` to resume after a disconnect. See [Reconnection and history](reconnection-and-history.md).
- `event`: the `type` field from the publish request, if any. Defaults to `message`. `EventSource` triggers `addEventListener("<type>", ...)` for non-default types.
- `data`: whatever the publisher sent in `data`. Mercure does not interpret it; it's bytes you decided on (JSON, HTML, JSON Patch, plain text...).

## Discovering the Mercure hub via link header

The publisher of a resource can advertise its hub via a `Link` header so clients don't need to hardcode it:

```http
# Discovering the Mercure Hub via Link Header
GET /books/1 HTTP/1.1
Host: example.com

200 OK
Content-Type: application/ld+json
Link: <https://hub.example.com/.well-known/mercure>; rel="mercure"
```

Subscribers that fetch the resource first can read the header to find the hub:

```javascript
// Discovering the Mercure Hub via Link Header
const res = await fetch("https://example.com/books/1");
const link = res.headers.get("Link");
const hub = link.match(/<([^>]+)>;\s*rel="?mercure"?/)[1];

const url = new URL(hub);
url.searchParams.append(
  "match",
  res.headers.get("Content-Location") ?? res.url,
);
new EventSource(url);
```

This pattern decouples the URL of your data from the URL of the hub serving updates for it. See [Discovery](discovery.md) for the full picture, including how clients learn the hub's authorization requirements.

## Mercure SSE heartbeats

The hub sends an SSE comment every `heartbeat` seconds (default `40s`). Heartbeats keep idle connections alive through proxies that close them after silence and let clients detect dead connections faster.

If you set `heartbeat 0s` to disable them, make sure nothing on the network path does idle-timeout TCP. Most CDNs and reverse proxies do.

## Mercure subscriber connection limits

| Limit                                      | Where                                      |
| ------------------------------------------ | ------------------------------------------ |
| Concurrent HTTP/2 streams per origin       | Browser-negotiated, default 100            |
| Concurrent HTTP/1.1 connections per origin | 6 (browser-enforced)                       |
| Concurrent connections to the hub          | Hardware-bound on OSS; tier-bound on Cloud |

A single connection serves any number of topics: pass more `match*` parameters rather than opening more `EventSource` instances.

## Next steps for Mercure subscribing

- [Topics and matchers](topics-and-matchers.md): choosing the right matcher type.
- [Reconnection and history](reconnection-and-history.md): surviving disconnects without losing events.
- [Active subscriptions](active-subscriptions.md): presence and the subscription API.
