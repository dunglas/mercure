---
title: "Mercure active subscriptions, presence, and subscription API"
description: "Track who is connected with Mercure subscription events and the JSON-LD subscription API for presence and live-collaboration UIs."
---

# Active subscriptions

The hub can act as its own publisher: every time a subscription is created or terminated, it publishes an update describing what happened. The hub also exposes a REST API for snapshotting the current set of subscriptions.

Together, they're how you build presence ("who's online?"), shared cursors ("who's looking at this document?"), and any feature that needs to react to other subscribers' comings and goings.

This feature is opt-in. Enable it in your Caddyfile:

```caddyfile
# Active Subscriptions
mercure {
  subscriptions
  # ...
}
```

## Subscription events

When the feature is on, the hub publishes a private update each time a subscription opens or closes. The topic follows this pattern:

```text
# Subscription events
/.well-known/mercure/subscriptions/{matchType}/{match}/{subscriber}
```

`{matchType}`, `{match}`, and `{subscriber}` are percent-encoded values. Subscribe to those topics with relative URL Patterns: the spec lets URL Patterns be relative to the hub URL, which is exactly what you want here:

```javascript
// Subscription events
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append(
  "matchURLPattern",
  "/.well-known/mercure/subscriptions/:matchType/:match/:subscriber",
);
new EventSource(url, { withCredentials: true });
```

Each event's `data` is a JSON-LD document:

```jsonc
// Subscription events
{
  "@context": "https://mercure.rocks/",
  "id": "/.well-known/mercure/subscriptions/URLPattern/https%3A%2F%2Fexample.com%2F%3Aselector/urn%3Auuid%3Abb3de268-05b0-4c65-b44e-8f9acefc29d6",
  "type": "Subscription",
  "matchType": "URLPattern",
  "match": "https://example.com/:selector",
  "subscriber": "urn:uuid:bb3de268-05b0-4c65-b44e-8f9acefc29d6",
  "active": true,
  "payload": { "username": "alice" },
}
```

Fields:

- `match`, `matchType`: the matcher the subscriber registered.
- `subscriber`: a hub-assigned identifier for the subscriber, shared by every subscription on the same connection.
- `active`: `true` for new subscriptions, `false` for terminated ones.
- `payload`: whatever the subscriber's token carried in the matching `subscribe` detail's `payload` (see [Authorization](authorization.md#subscriber-payloads)).

Subscription events are always **private**. To receive them, the listening subscriber's token needs a `subscribe` grant covering the `/.well-known/mercure/subscriptions/...` topic family.

## Authorization for subscription events

A typical "show me everyone subscribed to this document" feature is authorized like this:

```jsonc
// Authorization for subscription events
{
  "authorization_details": [
    {
      "type": "mercure",
      "actions": ["subscribe"],
      "topics": [
        {
          "match": "/.well-known/mercure/subscriptions/:matchType/:match/:subscriber",
          "matchType": "URLPattern",
        },
      ],
      "payload": { "username": "alice" },
    },
  ],
}
```

Tighten the matcher if a subscriber should only see presence for a specific document's topic:

```jsonc
// Authorization for subscription events
{
  "match": "/.well-known/mercure/subscriptions/:matchType/https%3A%2F%2Fexample.com%2Fdocs%2F42/:subscriber",
  "matchType": "URLPattern",
}
```

## Subscription API

Once subscription events are enabled, the hub also exposes a JSON-LD API. Use it to fetch the _current_ set of subscriptions when a client connects, then keep it in sync via subscription events.

| URL                                                                       | Returns                               |
| ------------------------------------------------------------------------- | ------------------------------------- |
| `GET /.well-known/mercure/subscriptions`                                  | All active subscriptions.             |
| `GET /.well-known/mercure/subscriptions/{matchType}/{match}`              | Subscriptions for a specific matcher. |
| `GET /.well-known/mercure/subscriptions/{matchType}/{match}/{subscriber}` | A single subscription.                |

Authorization rules are the same as for events: the request URL must be covered by a `subscribe` grant in the caller's token.

Each response carries `lastEventID`. Pass it to your SSE connection so you don't miss any subscription event between the snapshot and the live stream:

```javascript
// Subscription API
const snapshot = await fetch(
  "https://hub.example.com/.well-known/mercure/subscriptions",
  { credentials: "include" },
).then((r) => r.json());

const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append(
  "matchURLPattern",
  "/.well-known/mercure/subscriptions/:matchType/:match/:subscriber",
);
url.searchParams.append("lastEventID", snapshot.lastEventID);

const es = new EventSource(url, { withCredentials: true });
// snapshot.subscriptions is the initial list
// es.onmessage applies deltas as subscribers come and go
```

The hub returns:

```jsonc
// Subscription API
{
  "@context": "https://mercure.rocks/",
  "id": "/.well-known/mercure/subscriptions",
  "type": "Subscriptions",
  "lastEventID": "urn:uuid:5e94c686-2c0b-4f9b-958c-92ccc3bbb4eb",
  "subscriptions": [
    {
      "id": "/.well-known/mercure/subscriptions/URLPattern/https%3A%2F%2Fexample.com%2F%3Aselector/urn%3Auuid%3Abb3de268",
      "type": "Subscription",
      "match": "https://example.com/:selector",
      "matchType": "URLPattern",
      "subscriber": "urn:uuid:bb3de268",
      "active": true,
      "payload": { "username": "alice" },
    },
  ],
}
```

The data is volatile. Treat it as a cache, validate freshness, and don't rely on collection responses being complete forever. Terminated subscriptions may be omitted or kept with `active: false` depending on the hub's policy.

## The `subscriber` identifier

The hub assigns the `subscriber` identifier (a random `urn:uuid:`) when a subscription opens; clients cannot choose it. This keeps subscriber identity out of the token's control and avoids leaking a token's `sub` to other subscribers. Every subscription on the same connection shares the identifier, but a new connection (a reconnect, another tab, another device) gets a new one.

To attach a stable, human-meaningful identity to a subscriber, put it in the `subscribe` detail's `payload` (a username, a user URL, an avatar). The payload travels through subscription events, so peers see who is present without an extra round-trip, while the opaque `subscriber` value stays unguessable.

## Building presence with Mercure subscription events

A minimal presence panel:

1. On page load, fetch `/.well-known/mercure/subscriptions/{matchType}/{matchOfTheDocument}` to get who's currently here.
2. Open an SSE connection to subscription events for that topic.
3. On `active: true`, add the subscriber to the panel; on `active: false`, remove them.

Because the token's `subscribe` detail `payload` travels through subscription events, anything you put in there (username, avatar URL, role) is available to peers without an extra round-trip to your origin.

## Mercure subscription events performance

Subscription events are private updates like any other. They go through the hub's normal authorization pipeline. On a multi-thousand-subscriber hub with churn, the rate of subscription events can be significant; make sure the listeners that consume them have matchers narrow enough to receive only what they need.

## Disabling Mercure active subscriptions

If you don't need presence and want to save the cycles, leave `subscriptions` out of your Caddyfile (it's off by default). The hub then skips publishing subscription events and serves `404` on the subscription API URLs.
