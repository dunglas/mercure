---
title: "Mercure reconnection, last-event-id, and history buffer"
description: "How EventSource auto-reconnects with Last-Event-ID, how the Mercure hub replays missed updates from the history buffer, and how to detect data loss."
---

# Mercure reconnection and history

`EventSource` reconnects after connection loss. Mercure uses event IDs to replay missed updates while they remain available in the configured transport.

This page covers how the replay mechanism works, what it costs, and how to size the history buffer.

## Every event has an ID

The hub assigns a unique ID to each update (or echoes the one the publisher provided):

```text
id: urn:uuid:e1ee88e2-532a-4d6f-ba70-f0f8bd584022
event: message
data: {"status": "checked out"}

```

`EventSource` stores the most recently received `id` and sends it back in the `Last-Event-ID` HTTP header on reconnect. The hub uses it to find the right place in its history and replays everything after that ID before resuming the live stream.

## Bootstrapping after page load

A fresh `EventSource` does not remember a previous page's cursor. To cover updates published between reading a resource snapshot and opening the stream, return a cursor associated with that snapshot.

The publisher closes that gap by attaching a `last-event-id` attribute to its `Link` header at discovery time:

```http
GET /books/1
Host: example.com

200 OK
Link: <https://hub.example.com/.well-known/mercure>; rel="mercure"; last-event-id="urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb"
```

The subscriber adds the value to its first SSE request as a `last_event_id` query parameter:

```javascript
const hub = new URL("https://hub.example.com/.well-known/mercure");
hub.searchParams.append("match", "https://example.com/books/1");
hub.searchParams.append(
  "last_event_id",
  "urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb",
);
new EventSource(hub);
```

The hub replays everything published since that ID, then transitions to live updates. Browsers can't set HTTP headers on the first `EventSource` request, so the query parameter is the only option here. The header (`Last-Event-ID`) is what the browser uses on automatic reconnects.

## The `earliest` Mercure `last_event_id` value

Pass `last_event_id=earliest` to ask the hub for **everything it has** for the subscribed topics. The hub may decline this on policy grounds (it's a heavy request); when it accepts, you get the full history.

Use this to replay retained history. Keep authoritative state in your application database; the hub's history may be incomplete.

## Detecting data loss in Mercure replay

Whenever a request carries a resumption cursor, the hub sets the `Mercure-Last-Event-ID` HTTP **response** header to the ID of the event preceding the first one it actually sent, or `earliest` when there is no preceding event. By comparing what you asked for with what you got, you can tell whether you missed updates.

The response header is `Mercure-Last-Event-ID`; `Last-Event-ID` is the request header. This sketch assumes a mutable `expectedLastEventID`, an event `handler`, and an application `reloadSnapshot` function that restores state and starts a new subscription.

```javascript
// Native EventSource doesn't expose response headers; use fetch-event-source
import { fetchEventSource } from "@microsoft/fetch-event-source";

const controller = new AbortController();
await fetchEventSource(url, {
  signal: controller.signal,
  onopen: async (response) => {
    const replayedFrom = response.headers.get("Mercure-Last-Event-ID");
    if (replayedFrom !== expectedLastEventID) {
      controller.abort();
      await reloadSnapshot();
    }
  },
  headers: { "Last-Event-ID": expectedLastEventID },
  onmessage: (event) => {
    if (event.id) expectedLastEventID = event.id;
    handler(event);
  },
});
```

If a missing update would invalidate later patches, stop applying events when the cursor indicates a gap. Fetch a new snapshot, then resume from its cursor. Native `EventSource` cannot inspect this response header.

## The Mercure history buffer

The transport determines whether updates are retained. BoltDB stores history on disk; `transport local` has no replay history. BoltDB's `size 0` default disables automatic retention limits, so monitor disk use and configure a finite size when appropriate.

**Want us to manage the hub and its history? [Choose Mercure Cloud](https://mercure.rocks/pricing).** For shared history across your own hub instances, [Mercure Enterprise](../production/high-availability.md#self-hosted-transports) includes PostgreSQL, Kafka, Redis/Valkey, and Pulsar transports.

PostgreSQL keeps history in SQL tables alongside your application infrastructure. Kafka uses your broker's retention policy for replay across hub nodes. Choose an [Enterprise transport](../production/high-availability.md#picking-a-mercure-self-hosted-transport) to match the storage you already operate, or [talk to us about a Self-Hosted plan](https://mercure.rocks/pricing).

An update with [alternate topics](topics-and-matchers.md#alternate-topics) uses one BoltDB history record containing all its topics. Replay checks those topics against the subscriber's matchers.

### Configuring the Mercure BoltDB history size

```caddyfile
mercure {
  transport bolt {
    path /data/mercure.db
    size 1000000
    cleanup_frequency 0.3
  }
}
```

`size` sets the retention target. Cleanup runs probabilistically on publication, so the count can temporarily exceed it. `cleanup_frequency 0` disables cleanup.

A search for a specific event ID scans at most 10,000 recent events, or `size` events when larger. With `size 0`, an old event can remain stored but fall outside this search window. `earliest` replays the same window. Its `Mercure-Last-Event-ID` response remains `earliest`: IDs outside the replay window may belong to private or unrelated events. See [BoltDB configuration](../deployment/configuration.md#bolt-transport-default-single-node).

### Recovering when history is incomplete

Keep authoritative state in your application database. If replay cannot cover a gap, fetch a fresh snapshot and resume from its cursor. Set retention for the expected disconnect duration and total publication rate across the hub.

## Server-side Mercure reconnect behaviour

Publishers can set `retry` to suggest a reconnection delay in milliseconds:

```text
retry: 5000
```

The hub forwards this SSE field. If no delay is supplied, the client uses its own default. Browsers may apply additional backoff.

## Native `EventSource` doesn't expose response headers

This catches people. If you need to read the `Mercure-Last-Event-ID` response header (to detect data loss), you have to use a polyfill or library: `fetch-event-source` exposes it; native `EventSource` does not. Most server-side SSE clients also expose it.

## Header-based polyfills send the cursor as a query parameter

The `event-source-polyfill` package sends its reconnection cursor in a query parameter named `lastEventId` by default. This hub expects `last_event_id`, so configure that name explicitly. Other SSE clients may send the `Last-Event-ID` header instead.

Set the parameter name to `last_event_id`:

```javascript
new EventSourcePolyfill(url, {
  headers: { Authorization: `Bearer ${token}` },
  lastEventIdQueryParameterName: "last_event_id",
});
```

The polyfill then overwrites `last_event_id` with the last received ID on each reconnect, and the hub resumes from the right place.

## Common Mercure reconnect issues

| Symptom                                  | Cause                                                                                                                         |
| ---------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| Reconnects in a tight loop               | Token expired; mint a fresh one before reconnecting.                                                                          |
| Reconnect replays the same events        | Header-based polyfill uses its default `lastEventId` query parameter; set `lastEventIdQueryParameterName` to `last_event_id`. |
| Replay returns nothing                   | Event ID isn't in the hub's history (evicted, or hub doesn't have it yet).                                                    |
| Reconnect storm after a deploy           | The hub didn't drain gracefully. See [Rolling updates](../production/rolling-updates.md).                                     |
| Connections silently die after N minutes | Idle proxy timeout; lower `heartbeat` or extend the proxy's read timeout.                                                     |

[Troubleshooting](../production/troubleshooting.md) covers more.
