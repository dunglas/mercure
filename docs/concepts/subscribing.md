# Subscribing

A subscription is an HTTP `GET` request to the hub's well-known URL that the hub keeps open and writes [Server-Sent Events](https://html.spec.whatwg.org/multipage/server-sent-events.html) into.

```http
GET /.well-known/mercure?match=https://example.com/books/1 HTTP/2
Host: hub.example.com
Accept: text/event-stream
```

Pick the matchers you want with [`match*` query parameters](topics-and-matchers.md). The rest of this page covers the client side.

## Browser

`EventSource` is built into every modern browser:

```javascript
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append("match", "https://example.com/books/1");
url.searchParams.append("matchURLPattern", "https://example.com/users/:id");

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

- Browsers cap concurrent HTTP/1.1 requests per origin at 6. With HTTP/2 (the default everywhere on HTTPS) the cap is 100 streams negotiated with the server. **Use HTTP/2** — your hub already speaks it.
- A single `EventSource` connection can carry as many topic subscriptions as you want by passing more `match*` parameters.
- `EventSource` does not let you set `Authorization` headers. For private subscriptions, use a [cookie](authorization.md#cookies-in-detail) (recommended) or the `authorization` query parameter (last resort).

### `fetch-event-source` for advanced cases

For finer-grained error handling, custom headers, or non-`GET` requests, use [Microsoft's `fetch-event-source`](https://github.com/Azure/fetch-event-source):

```javascript
import { fetchEventSource } from "@microsoft/fetch-event-source";

await fetchEventSource(url, {
  headers: { Authorization: `Bearer ${jwt}` },
  onmessage: (event) => /* ... */,
  onerror: (err) => /* ... */,
});
```

This is the right choice for React Native, server-side runtimes, and any case where the native `EventSource` is too constrained.

## Closing the connection

`EventSource` keeps the TCP connection open until you close it. In single-page apps the connection survives component unmounts and route changes if you don't tear it down explicitly:

```javascript
useEffect(() => {
  const es = new EventSource(url);
  es.onmessage = handler;
  return () => es.close();
}, [url]);
```

Skipping this is the most common cause of "the hub keeps a slot for me even though I navigated away." The hub sees the connection as live and counts it against any per-IP or per-token limits.

## Server-side subscribers

Any HTTP client that exposes a streaming response works. A few examples:

### Node.js

```javascript
import { fetchEventSource } from "@microsoft/fetch-event-source";

await fetchEventSource("https://hub.example.com/.well-known/mercure?match=topic", {
  onmessage: (e) => console.log(e.data),
});
```

### Go

```go
import "github.com/r3labs/sse/v2"

client := sse.NewClient("https://hub.example.com/.well-known/mercure?match=topic")
client.Subscribe("messages", func(msg *sse.Event) {
    fmt.Println(string(msg.Data))
})
```

### Python

```python
from sseclient import SSEClient

for event in SSEClient("https://hub.example.com/.well-known/mercure?match=topic"):
    print(event.data)
```

[Awesome Mercure](../ecosystem/awesome.md) lists more libraries.

## What the hub sends

Each event is a standard SSE message:

```
id: urn:uuid:e1ee88e2-532a-4d6f-ba70-f0f8bd584022
event: message
data: {"status": "checked out"}

```

Fields:

- `id` — a unique identifier the hub assigns to every update. Clients send it back in `Last-Event-ID` to resume after a disconnect. See [Reconnection and history](reconnection-and-history.md).
- `event` — the `type` field from the publish request, if any. Defaults to `message`. `EventSource` triggers `addEventListener("<type>", ...)` for non-default types.
- `data` — whatever the publisher sent in `data`. Mercure does not interpret it; it's bytes you decided on (JSON, HTML, JSON Patch, plain text...).

## Discovering the hub

The publisher of a resource can advertise its hub via a `Link` header so clients don't need to hard-code it:

```http
GET /books/1 HTTP/1.1
Host: example.com

200 OK
Content-Type: application/ld+json
Link: <https://hub.example.com/.well-known/mercure>; rel="mercure"
```

Subscribers that fetch the resource first can read the header to find the hub:

```javascript
const res = await fetch("https://example.com/books/1");
const link = res.headers.get("Link");
const hub = link.match(/<([^>]+)>;\s*rel="?mercure"?/)[1];

const url = new URL(hub);
url.searchParams.append("match", res.headers.get("Content-Location") ?? res.url);
new EventSource(url);
```

This pattern decouples the URL of your data from the URL of the hub serving updates for it.

## Heartbeats

The hub sends an SSE comment every `heartbeat` seconds (default `40s`). Heartbeats keep idle connections alive through proxies that close them after silence and let clients detect dead connections faster.

If you set `heartbeat 0s` to disable them, make sure nothing on the network path does idle-timeout TCP — most CDNs and reverse proxies do.

## Connection limits

| Limit | Where |
| --- | --- |
| Concurrent HTTP/2 streams per origin | Browser-negotiated, default 100 |
| Concurrent HTTP/1.1 connections per origin | 6 (browser-enforced) |
| Concurrent connections to the hub | Hardware-bound on OSS; tier-bound on Cloud |

A single connection serves any number of topics — pass more `match*` parameters rather than opening more `EventSource` instances.

## Next

- [Topics and matchers](topics-and-matchers.md) — choosing the right matcher type.
- [Reconnection and history](reconnection-and-history.md) — surviving disconnects without losing events.
- [Active subscriptions](active-subscriptions.md) — presence and the subscription API.
