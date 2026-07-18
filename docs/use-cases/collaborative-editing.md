---
title: "Build collaborative editing on Mercure with CRDTs"
description: "Combine Mercure broadcast, presence, and replay with Yjs or Automerge to build real-time collaborative editing features."
---

# Collaborative editing

Multiple users edit the same document and see each other's changes in real time. This guide covers the pieces Mercure handles directly (broadcast, presence, replay) and outlines where you still need to bring your own logic (conflict resolution).

## Pieces of a collaborative editor on Mercure

A working collaborative editor needs:

1. **Broadcast.** When user A makes a change, every other connected user sees it.
2. **Conflict resolution.** Two users editing the same span at once should converge to the same result.
3. **Presence.** Show who else is here and where their cursor is.
4. **Persistence.** The document survives page reloads.
5. **Late join.** A user opening the doc mid-session sees the latest state, not just the next change.

Mercure handles 1, 3, and partially 5. Bring a CRDT (Yjs, Automerge, Loro) for 2. Use your normal database for 4.

## Broadcasting document changes via Mercure

Each document has a topic; every change is published to it.

```javascript
// Connect to the document
const docId = "books/42";
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append("match", `https://docs.example.com/${docId}`);
url.searchParams.append(
  "match_urlpattern",
  `/.well-known/mercure/subscriptions/:match_type/:match/:subscriber`,
);

const es = new EventSource(url, { withCredentials: true });
```

When the local user types:

```javascript
// Broadcasting Document Changes via Mercure
editor.on("change", async (delta) => {
  await fetch("/api/docs/" + docId + "/change", {
    method: "POST",
    body: JSON.stringify(delta),
    headers: { "Content-Type": "application/json" },
  });
});
```

The origin server stores the delta in the document's history and publishes it to the hub:

```python
# Broadcasting Document Changes via Mercure
def post_change(doc_id: str, delta: dict, user_id: str) -> None:
    db.append_change(doc_id, delta, user_id)
    publish(
        topic=f"https://docs.example.com/{doc_id}",
        data=json.dumps({"type": "delta", "delta": delta, "author": user_id}),
    )
```

Every connected client receives the delta and applies it to their local copy.

## CRDT conflict resolution with Mercure transport

Mercure delivers messages; it does not order them across publishers. If you publish raw text edits ("insert 'h' at position 5"), two users typing at once will produce inconsistent results.

The standard answer is a **CRDT**: a data structure that lets local edits commute. The most popular library is [Yjs](https://yjs.dev/). The integration looks like:

```javascript
// CRDT Conflict Resolution with Mercure Transport
import * as Y from "yjs";

const ydoc = new Y.Doc();
const text = ydoc.getText("content");

ydoc.on("update", (update, origin) => {
  if (origin === "remote") return; // don't echo
  fetch("/api/docs/" + docId + "/change", {
    method: "POST",
    body: update, // a Yjs binary update
    headers: { "Content-Type": "application/octet-stream" },
  });
});

es.onmessage = (event) => {
  const update = base64ToBytes(JSON.parse(event.data).update);
  Y.applyUpdate(ydoc, update, "remote");
};
```

The CRDT guarantees convergence; Mercure just ferries the binary updates around.

[Automerge](https://automerge.org/) and [Loro](https://www.loro.dev/) work the same way. Mercure doesn't care what's in the payload.

## Collaborative presence with Mercure subscription events

Use [subscription events](../concepts/active-subscriptions.md) to show who's connected. Each user's access token carries a payload with their name and color:

```jsonc
// Collaborative Presence with Mercure Subscription Events (header: { "alg": "...", "typ": "at+jwt" })
{
  "iss": "https://example.com",
  "aud": "https://hub.example.com/.well-known/mercure",
  "exp": 4102444800,
  "authorization_details": [
    {
      "type": "https://mercure.rocks/authorization-detail",
      "actions": ["subscribe"],
      "topics": [
        { "match": "https://docs.example.com/books/42" },
        {
          "match": "/.well-known/mercure/subscriptions/:match_type/:match/:subscriber",
          "match_type": "urlpattern",
        },
      ],
      "payload": { "name": "Alice", "color": "#ff0066" },
    },
  ],
}
```

The hub assigns each connection a random `urn:uuid:` subscriber ID. For a stable identity in presence, surface the `payload` (here `name` and `color`) rather than relying on a client-chosen subscriber ID.

When subscriptions on the document topic open or close, the hub broadcasts an event including that payload. The UI maintains a list of "people here":

```javascript
// Collaborative Presence with Mercure Subscription Events
const peers = new Map();

const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append(
  "match_urlpattern",
  "/.well-known/mercure/subscriptions/:match_type/:match/:subscriber",
);

new EventSource(url, { withCredentials: true }).onmessage = (event) => {
  const sub = JSON.parse(event.data);
  if (sub.match !== "https://docs.example.com/books/42") return;
  if (sub.active) {
    peers.set(sub.subscriber, sub.payload);
  } else {
    peers.delete(sub.subscriber);
  }
  renderPeers(peers);
};
```

For cursor positions and selections, publish them on a separate topic per peer (so they don't pollute the document's change stream):

```javascript
// Collaborative Presence with Mercure Subscription Events
fetch("/api/docs/" + docId + "/cursor", {
  method: "POST",
  body: JSON.stringify({ from, to }),
});
```

```python
# Collaborative Presence with Mercure Subscription Events
publish(
  topic=f"https://docs.example.com/{doc_id}/cursors/{user_id}",
  data=json.dumps({"from": from_pos, "to": to_pos}),
)
```

## Late join: hydrating a collaborative document

A user opens the doc and needs the latest state, not just the next change.

The simplest approach: serve the current document body as a normal HTTP `GET`, then subscribe to changes from the event ID returned in the `Link: rel=mercure` header's `last-event-id` attribute. See [Reconnection and history](../concepts/reconnection-and-history.md#bootstrapping-after-page-load).

For CRDT documents, hydrate the local Yjs doc from the snapshot and replay only the deltas published since.

## Persisting collaborative document state

Mercure is a real-time bus, not a database. Persist:

- The document's content (or its CRDT state) in your application's database.
- Per-change history if you want undo/redo or audit trails.

The hub's history buffer is for surviving brief disconnects, not for storing months of revisions. (You _can_ configure it to do that, especially with the Postgres transport, but a database is the right tool for the job.)

## Authorization

Documents are usually private. Each user's `subscribe` grant should cover only the documents they have access to:

```jsonc
// Authorization (header: { "alg": "...", "typ": "at+jwt" })
{
  "iss": "https://example.com",
  "aud": "https://hub.example.com/.well-known/mercure",
  "exp": 4102444800,
  "authorization_details": [
    {
      "type": "https://mercure.rocks/authorization-detail",
      "actions": ["subscribe"],
      "topics": [
        { "match": "https://docs.example.com/books/42" },
        {
          "match": "https://docs.example.com/books/42/cursors/:user",
          "match_type": "urlpattern",
        },
        {
          "match": "/.well-known/mercure/subscriptions/:match_type/:match/:subscriber",
          "match_type": "urlpattern",
        },
      ],
    },
  ],
}
```

Mark every change publication `private=on` so the hub enforces the claim.

## What about the publish path?

If you want clients to publish directly to the hub (skipping the origin server), you can: give them a publisher JWT. But typically you don't. Routing changes through your API lets you persist them, validate them, and rate-limit them. The publish to Mercure is just a fan-out at the end of that pipeline.

> **Pro tip.** Collaborative apps benefit from multi-region deployments. The open-source hub runs on a single node; for HA across regions you'll want [Self-Hosted Mercure](https://mercure.rocks/pricing) with Redis or Postgres transports, or the [managed Cloud version](https://mercure.rocks/pricing).

## Next steps for collaborative editing on Mercure

- [Active subscriptions](../concepts/active-subscriptions.md): the presence layer in detail.
- [Reconnection and history](../concepts/reconnection-and-history.md): surviving disconnects.
- [Hotwire](hotwire.md): when "the document" is HTML and you want to broadcast HTML diffs.
