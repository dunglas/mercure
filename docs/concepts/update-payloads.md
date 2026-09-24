---
title: "Structuring Mercure update payloads with ActivityStreams 2.0"
description: "Choose an envelope for the Mercure data field so subscribers can tell creations from updates and deletions, with ActivityStreams 2.0 as a worked example."
---

# Mercure update payloads

Mercure carries UTF-8 text in the `data` field. The hub forwards it without interpreting the application format. Use JSON, HTML, plain text, or encode binary data as text.

Define how clients distinguish creation, modification, and deletion. You can put that information in the payload or use the publish `type` field, which becomes the SSE event name.

## The bare payload

The rest of this documentation uses the simplest thing that works: the new state of the resource, as JSON.

```console
curl -X POST https://hub.example.com/.well-known/mercure \
  -H "Authorization: Bearer $JWT" \
  --data-urlencode 'topic=https://example.com/books/1' \
  --data-urlencode 'data={"status": "checked out"}'
```

A bare payload works when both sides agree on its meaning. For example, replace a resource with a full snapshot, apply specified changed fields, or refetch the resource. Use an envelope when consumers need event metadata.

## Choosing an envelope

| Format                                                             | Purpose                                                                                                               |
| ------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------- |
| Bare JSON                                                          | Nothing to agree on beyond the resource shape. Smallest payload, no create/update/delete distinction.                 |
| [JSON Patch](https://www.rfc-editor.org/rfc/rfc6902)               | Partial updates: send the diff instead of the whole resource. Subscriber must already hold the current state.         |
| [JSON Merge Patch](https://www.rfc-editor.org/rfc/rfc7396)         | Partial updates with a simpler syntax than JSON Patch, arrays are replaced as a whole, and `null` removes a property. |
| [CloudEvents](https://cloudevents.io/)                             | Transport-agnostic event metadata (`source`, `type`, `time`) shared with your queues and functions.                   |
| [ActivityStreams 2.0](https://www.w3.org/TR/activitystreams-core/) | A W3C vocabulary for the change itself: `Create`, `Update`, `Delete`, `Add`, `Remove` over an `object`.               |

Pick one per topic and stick to it. Subscribers cannot sniff the format reliably, and mixing envelopes on a single topic forces every consumer to guess.

## ActivityStreams 2.0

[ActivityStreams 2.0](https://www.w3.org/TR/activitystreams-core/) is a W3C Recommendation that models an event as an _activity_: a `type` describing what happened, and an `object` it happened to. It fits Mercure well because it answers exactly the question the topic can't.

```console
curl -X POST https://hub.example.com/.well-known/mercure \
  -H "Authorization: Bearer $JWT" \
  --data-urlencode 'topic=https://example.com/books/1' \
  --data-urlencode 'data={
    "@context": "https://www.w3.org/ns/activitystreams",
    "id": "https://example.com/activities/8f2c",
    "type": "Update",
    "published": "2026-01-15T14:03:11Z",
    "object": {
      "id": "https://example.com/books/1",
      "type": "Document",
      "name": "Zen and the Art of Motorcycle Maintenance",
      "https://example.com/vocab/status": "checked out"
    }
  }'
```

A deletion becomes an activity of its own, on the same topic, so a subscriber that only ever sees the payload still knows the resource is gone:

```json
{
  "@context": "https://www.w3.org/ns/activitystreams",
  "type": "Delete",
  "object": "https://example.com/books/1"
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

ActivityStreams `Update` identifies a change but does not define your application's merge rules. Specify whether the embedded object is a full snapshot or a partial representation. The example above uses a snapshot.

Two workable answers:

- **Send the full object.** Clients can replace their local snapshot. Costs bandwidth on large resources.
- **Send a diff and say so.** Put a [JSON Patch](https://www.rfc-editor.org/rfc/rfc6902) document in the payload and give it its own activity type in your namespace, rather than overloading `Update`.

Document the update semantics for each topic so clients know whether to replace or merge state.

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

## Next steps

- [Publishing](publishing.md): the form fields and the publish-side clients.
- [Subscribing](subscribing.md): what the SSE frame around the payload looks like.
- [Reconnection and history](reconnection-and-history.md): replay, and why activities can arrive twice.
- [Encryption](encryption.md): the envelope goes inside the JWE payload, so the hub sees neither.
