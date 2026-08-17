---
title: "Structuring Mercure update payloads with ActivityStreams 2.0"
description: "Choose an envelope for the Mercure data field so subscribers can tell creations from updates and deletions, with ActivityStreams 2.0 as a worked example."
---

# Update payloads

The hub treats `data` as opaque bytes. It stores and forwards whatever you post without parsing it, so the shape of the payload is a convention between the publisher and the subscriber. Nothing on this page is required by the protocol, and the hub will never validate or reject a payload for its structure.

What the protocol does _not_ give you is a way to say **what happened**. An update on `https://example.com/books/1` could mean the book was created, its status changed, or it was deleted, and the topic alone doesn't distinguish them. That distinction has to live in the payload.

## The bare payload

The rest of this documentation uses the simplest thing that works: the new state of the resource, as JSON.

```console
# The bare payload
curl -X POST https://hub.example.com/.well-known/mercure \
  -H "Authorization: Bearer $JWT" \
  -d 'topic=https://example.com/books/1' \
  -d 'data={"status": "checked out"}'
```

This is enough when the topic identifies exactly one resource and the subscriber replaces its local copy wholesale (or refetches the resource and uses the update purely as a signal). Reach for an envelope when it isn't: when the same topic carries several kinds of change, when deletions have to be distinguishable from updates, or when consumers you don't control need to route on the payload.

## Choosing an envelope

| Format                                                             | What it buys you                                                                                                       |
| ------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------- |
| Bare JSON                                                          | Nothing to agree on beyond the resource shape. Smallest payload, no create/update/delete distinction.                  |
| [JSON Patch](https://www.rfc-editor.org/rfc/rfc6902)               | Partial updates: send the diff instead of the whole resource. Subscriber must already hold the current state.          |
| [JSON Merge Patch](https://www.rfc-editor.org/rfc/rfc7386)         | Partial updates with a simpler syntax than JSON Patch, at the cost of not being able to express array edits or `null`. |
| [CloudEvents](https://cloudevents.io/)                             | Transport-agnostic event metadata (`source`, `type`, `time`) shared with your queues and functions.                    |
| [ActivityStreams 2.0](https://www.w3.org/TR/activitystreams-core/) | A W3C vocabulary for the change itself: `Create`, `Update`, `Delete`, `Add`, `Remove` over an `object`.                |

Pick one per topic and stick to it. Subscribers cannot sniff the format reliably, and mixing envelopes on a single topic forces every consumer to guess.

## ActivityStreams 2.0

[ActivityStreams 2.0](https://www.w3.org/TR/activitystreams-core/) is a W3C Recommendation that models an event as an _activity_: a `type` describing what happened, and an `object` it happened to. It fits Mercure well because it answers exactly the question the topic can't.

```console
# ActivityStreams 2.0
curl -X POST https://hub.example.com/.well-known/mercure \
  -H "Authorization: Bearer $JWT" \
  -d 'topic=https://example.com/books/1' \
  --data-urlencode 'data={
    "@context": "https://www.w3.org/ns/activitystreams",
    "id": "https://example.com/activities/8f2c",
    "type": "Update",
    "published": "2026-01-15T14:03:11Z",
    "object": {
      "id": "https://example.com/books/1",
      "type": "Document",
      "name": "Zen and the Art of Motorcycle Maintenance",
      "status": "checked out"
    }
  }'
```

A deletion becomes an activity of its own, on the same topic, so a subscriber that only ever sees the payload still knows the resource is gone:

```jsonc
// A deletion
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "type": "Delete",
  "object": "https://example.com/books/1",
}
```

On the subscriber, switch on `type`:

```javascript
// On the subscriber
const es = new EventSource(url);
es.onmessage = (event) => {
  const activity = JSON.parse(event.data);
  switch (activity.type) {
    case "Create":
    case "Update":
      books.set(activity.object.id, activity.object);
      break;
    case "Delete":
      books.delete(activity.object.id ?? activity.object);
      break;
  }
};
```

### Full replacement vs. partial updates

In ActivityStreams, `Update` describes a change to the object as a whole, and the `object` is the object's new state. It has no notion of a diff, which is the single sharpest edge of using it for real-time updates: publishers that send only the changed fields are relying on a convention the vocabulary does not define.

Two workable answers:

- **Send the full object.** Simplest, and what the vocabulary means. Costs bandwidth on large resources.
- **Send a diff and say so.** Put a [JSON Patch](https://www.rfc-editor.org/rfc/rfc6902) document in the payload and give it its own activity type in your namespace, rather than overloading `Update`.

Fix the choice per topic. A subscriber that has to work out whether `object` is a full resource or a patch on every message will get it wrong eventually.

### Two kinds of identifier

An ActivityStreams payload carries its own `id`, and so does the SSE frame around it. They are not the same thing and neither substitutes for the other:

- The activity's `id` and `published` belong to the publisher. They identify the activity in the publisher's own domain, survive replays, and mean nothing to the hub.
- The SSE `id` is assigned by the hub (unless you set the publish `id` field yourself). It's what a subscriber sends back in `Last-Event-ID` to resume a stream. See [Reconnection and history](reconnection-and-history.md).

Deduplicating on the activity `id` is worth doing if your publisher can retry a publication: `Last-Event-ID` replay can legitimately deliver the same activity twice.

### Media types

ActivityStreams normally travels as `application/activity+json` (or `application/ld+json` with the ActivityStreams profile). The Mercure `data` field carries no media type of its own, so there is nowhere to put that.

Two ways to signal the format anyway:

- **Out of band.** Document it. If a topic always carries ActivityStreams, the subscriber knows without being told at runtime.
- **Via the publish `type` field.** It becomes the SSE `event` name, and `EventSource` dispatches it to `addEventListener("<name>", ...)`. Useful when one topic mixes formats, though a topic that mixes formats is usually a topic that should have been split.

The `@context` in the payload is what actually makes it ActivityStreams to a JSON-LD consumer, and it's worth keeping even when both ends are yours.

## Mercure and ActivityPub

[ActivityPub](https://www.w3.org/TR/activitypub/) builds on ActivityStreams to federate servers, and a Mercure hub pairs naturally with it: your ActivityPub server keeps handling federation, and publishes the activities it accepts to the hub so browser clients get them live instead of polling the outbox.

The hub is not itself an ActivityPub implementation. It has no actors, no inbox or outbox, no collection paging, and does not verify [HTTP Signatures](https://www.w3.org/TR/activitypub/#authorization). It moves activities to connected clients; everything federation-facing stays in your application.

## Next steps for Mercure update payloads

- [Publishing](publishing.md): the form fields and the publish-side clients.
- [Subscribing](subscribing.md): what the SSE frame around the payload looks like.
- [Reconnection and history](reconnection-and-history.md): replay, and why activities can arrive twice.
- [Encryption](encryption.md): the envelope goes inside the JWE payload, so the hub sees neither.
