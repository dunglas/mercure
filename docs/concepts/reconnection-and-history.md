# Reconnection and History

Network connections drop. SSE clients reconnect automatically — Mercure adds a way to **resume from the last event you saw**, so a brief disconnect doesn't lose updates.

This page covers how the replay mechanism works, what it costs, and how to size the history buffer.

## Every event has an ID

The hub assigns a unique ID to each update (or echoes the one the publisher provided):

```
id: urn:uuid:e1ee88e2-532a-4d6f-ba70-f0f8bd584022
event: message
data: {"status": "checked out"}

```

`EventSource` stores the most recently received `id` and sends it back in the `Last-Event-ID` HTTP header on reconnect. The hub uses it to find the right place in its history and replays everything after that ID before resuming the live stream.

## Bootstrapping after page load

The reconnection mechanism only solves *gap during a session*. The other gap to defend against is the one between **when your server generated the page** and **when the browser opened the SSE connection** — anywhere from a few hundred milliseconds to several seconds, during which updates may have been published.

The publisher closes that gap by attaching a `last-event-id` attribute to its `Link` header at discovery time:

```http
GET /books/1 HTTP/1.1
Host: example.com

200 OK
Link: <https://hub.example.com/.well-known/mercure>; rel="mercure"; last-event-id="urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb"
```

The subscriber adds the value to its first SSE request as a `lastEventID` query parameter:

```javascript
const hub = new URL("https://hub.example.com/.well-known/mercure");
hub.searchParams.append("match", "https://example.com/books/1");
hub.searchParams.append("lastEventID", "urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb");
new EventSource(hub);
```

The hub replays everything published since that ID, then transitions to live updates. Browsers can't set HTTP headers on the first `EventSource` request, so the query parameter is the only option here. The header (`Last-Event-ID`) is what the browser uses on automatic reconnects.

## The `earliest` value

Pass `lastEventID=earliest` to ask the hub for **everything it has** for the subscribed topics. The hub may decline this on policy grounds (it's a heavy request); when it accepts, you get the full history.

This is the right way to seed an event-sourced view from the hub.

## Detecting data loss

If the requested event ID is no longer in the hub's history (it was evicted), the hub sets the `Last-Event-ID` HTTP **response** header to the ID of the event preceding the first one it actually sent. By comparing what you asked for with what you got, you can tell whether you missed updates.

```javascript
// Native EventSource doesn't expose response headers; use fetch-event-source
import { fetchEventSource } from "@microsoft/fetch-event-source";

await fetchEventSource(url, {
  onopen: (response) => {
    const replayedFrom = response.headers.get("Last-Event-ID");
    if (replayedFrom !== expectedLastEventID) {
      // Possibly missed events — refetch the resource from the origin
    }
  },
  onmessage: handler,
});
```

For partial-update streams (JSON Patch, JSON Merge Patch) or anything where missing one update breaks the next one, **always check this header**. For idempotent full-state pushes, you can usually skip it.

## The history buffer

The hub stores recent events in a transport. The size of that buffer determines how far back a subscriber can replay.

| Build | Default transport | History capacity |
| --- | --- | --- |
| Open-source hub | BoltDB | **Unlimited** (bound by disk space) |
| Cloud (Free) | Managed | None (no replay) |
| Cloud (Hobby) | Managed | 100 messages |
| Cloud (Pro) | Managed | 500 messages |
| Cloud (Business) | Managed | 5,000 messages |
| Self-Hosted (any tier) | Redis / PostgreSQL / Kafka / Pulsar | **Unlimited** (bound by your storage) |

> **Pro tip.** The open-source hub has **no built-in history limit**. The Cloud caps exist for operational reasons — managed instances need predictable storage. If you're running on your own infrastructure and want to keep weeks of history for replay or event sourcing, the open-source build will store everything you give it disk for.

### Configuring BoltDB history

By default, the BoltDB transport keeps everything. To put a cap on it:

```caddyfile
transport bolt {
  path /data/mercure.db
  size 1000000        # keep at most 1M events
  cleanup_frequency 0.3
}
```

`cleanup_frequency` is the chance (between 0 and 1) of running a cleanup pass on each publish. The default `0.3` strikes a balance between write latency and storage growth. See [Configuration](../deployment/configuration.md#bolt-transport-default-single-node).

### When history isn't enough

For workflows where lost updates are unacceptable — partial updates that mutate state, primary event store — pair the hub with a durable system:

- **Use a primary store.** Persist the source of truth (Postgres, your domain DB) and treat Mercure as the live broadcast. On reconnect with data loss, refetch from the store.
- **Use the PostgreSQL transport.** Self-Hosted ships a transport that stores events in Postgres. You can then query them with SQL alongside your application data.
- **Keep events forever.** Set `size 0` on BoltDB or rely on Postgres/Kafka retention.

## Server-side reconnect behavior

The hub sets a `retry` field on the SSE stream:

```
retry: 5000

```

Browsers wait at least that many milliseconds before reconnecting after a disconnect. The hub picks a sensible default; override it if you need a different cadence.

## Native `EventSource` doesn't expose response headers

This catches people. If you need to read the `Last-Event-ID` response header (to detect data loss), you have to use a polyfill or library — `fetch-event-source` exposes it, native `EventSource` does not. Most server-side SSE clients also expose it.

## Common reconnect issues

| Symptom | Cause |
| --- | --- |
| Reconnects in a tight loop | Token expired; mint a fresh one before reconnecting. |
| Replay returns nothing | Event ID isn't in the hub's history (evicted, or hub doesn't have it yet). |
| Reconnect storm after a deploy | The hub didn't drain gracefully. See [Rolling updates](../production/rolling-updates.md). |
| Connections silently die after N minutes | Idle proxy timeout; lower `heartbeat` or extend the proxy's read timeout. |

[Troubleshooting](../production/troubleshooting.md) covers more.
