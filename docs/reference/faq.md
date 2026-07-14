---
title: "Mercure FAQ: WebSockets, Pusher, GraphQL, and connection limits"
description: "Common Mercure questions: comparisons with WebSockets, Pusher, Ably, WebSub, and Web Push, and answers about connection limits and history."
---

# FAQ

## What's the difference between Mercure and WebSockets?

Mercure is a high-level protocol; WebSocket is a low-level transport. Mercure ships with authorization, reconnection, replay, and presence built in: features you'd otherwise design from scratch on top of WebSocket.

WebSocket is also [hard to scale on HTTP/2+](https://www.infoq.com/articles/websocket-and-http2-coexist) and [hard to secure](https://gravitational.com/blog/kubernetes-websocket-upgrade-security-vulnerability/). Mercure rides on plain HTTP/2 (or 3), so it inherits the multiplexing and security properties of the underlying transport.

For most server-pushes-to-client use cases, Mercure is a smaller, easier replacement. WebSocket still wins when you need a tightly-bound, low-latency, full-duplex channel inside one connection: game state, voice, intricate collaborative cursors. For everything else, the round-trip cost of "subscribe via SSE, send via `POST`" is negligible on HTTP/2.

## What's the difference between Mercure and Pusher / Ably / Firebase Realtime Database?

Two big ones:

- **Mercure is open.** It's a protocol with an open-source reference hub. Pusher, Ably, and Firebase are SaaS-only: your data and connections live on their infrastructure, and migration means rewriting against a different vendor.
- **The free tier is the production tier.** The open-source hub has unlimited connections and an unlimited history buffer. SaaS competitors meter both.

We do offer paid tiers ([Cloud and Self-Hosted](https://mercure.rocks/pricing)), but they're for users who specifically want managed infrastructure or multi-node setups, not because the open-source build is crippled.

## What's the difference between Mercure and WebSub?

[WebSub](https://www.w3.org/TR/websub/) is server-to-server only. Mercure does server-to-server, server-to-client, and client-to-client over the same primitive.

Mercure was inspired by WebSub and stays close to it where it can. The big differences: Mercure uses SSE rather than `POST` callbacks, ships JWT-based authorization, and supports per-connection multi-topic subscriptions.

## What's the difference between Mercure and Web Push?

The [Push API](https://developer.mozilla.org/en-US/docs/Web/API/Push_API) targets _offline_ devices, with notifications routed through Apple, Google, or Mozilla's push servers. The payload is small, the delivery semantics are at-most-once, and the audience is OS notification centers.

Mercure targets _connected_ clients. Payloads are unbounded, deliveries are ordered with replay, and there's no third party in the path.

Use Web Push for "tap to wake the user up." Use Mercure for "the user is already in the app."

## How many connections can a single hub hold?

Public benchmarks: 40k concurrent on a t3.micro, well over 100k on bigger instances. Connection count is not the binding constraint for Mercure. Fan-out throughput (publish x matching subscribers) usually saturates the network before connection count saturates RAM.

For setups beyond a single node, see [High availability](../production/high-availability.md).

## What's the maximum number of open connections per browser?

On HTTP/2 (default everywhere on HTTPS), the browser negotiates with the server. Default cap is 100 streams per origin. On HTTP/1.1, it's 6.

A single `EventSource` connection can serve as many topic subscriptions as you want via multiple `match*` query parameters, so this limit rarely matters in practice.

## Can a single subscriber be in many topics over one connection?

Yes. Pass several `match*` parameters:

```javascript
// Can a single subscriber be in many topics over one connection?
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append("match", "https://example.com/announcements");
url.searchParams.append(
  "match_urlpattern",
  "https://example.com/users/:id/notifications",
);
new EventSource(url);
```

One TCP connection. Any number of subscriptions.

## How do I use Mercure with GraphQL?

Mercure is delivery-agnostic, so it pairs cleanly with GraphQL subscriptions: the GraphQL server returns a topic in response to a subscription query, the client opens an `EventSource` on that topic. See [GraphQL subscriptions](../use-cases/graphql.md). The [API Platform framework](https://api-platform.com/docs/master/core/graphql/#subscriptions) ships this integration out of the box.

## How do I use Mercure with Hotwire / Turbo Streams?

Connect a Turbo Stream source to an `EventSource`:

```javascript
// How do I use Mercure with Hotwire / Turbo Streams?
import { connectStreamSource } from "@hotwired/turbo";
connectStreamSource(new EventSource(mercureUrl));
```

Publish HTML with `<turbo-stream>` tags as the `data` field. See [Hotwire / Turbo Streams](../use-cases/hotwire.md).

## How do I send the authorization cookie to the hub?

For cross-origin `EventSource`, set `withCredentials`:

```javascript
// How do I send the authorization cookie to the hub?
new EventSource(url, { withCredentials: true });
```

The hub must respond with the right CORS headers (`cors_origins` listing the calling origin, no wildcard). See [Authorization](../concepts/authorization.md#cookies-in-detail).

## Can I run Mercure without a hub?

Technically yes: the protocol allows applications to deliver SSE directly. In practice, the only people doing this are framework authors who embed the hub inside their stack ([FrankenPHP](https://frankenphp.dev), for instance). For everyone else, run the hub.

## Does Mercure work with serverless?

Yes, on the publisher side: a Lambda or Cloud Function can `POST` to the hub and exit. On the subscriber side, the hub is the long-lived process; your serverless functions don't have to keep connections open.

If you're thinking about running the hub itself on serverless: the hub holds long-lived connections, so platforms with execution timeouts (most serverless) don't fit. Run the hub somewhere with persistent compute.

## What's the Mercure hub delivery latency?

End to end, the median publish-to-deliver latency on a single-node hub is sub-millisecond plus your network. With multi-node Self-Hosted transports, expect single-digit milliseconds added by the transport.

## How do I monitor it?

Prometheus metrics on the admin port, plus the `mercure_subscribers_connected` and `mercure_updates_total` series for the most useful signals. Full list in [Health monitoring](../production/health-monitoring.md).

## What about messages I publish before any subscriber connects?

The hub stores them in its history buffer. A subscriber connecting later can replay them by passing `last_event_id=earliest` (gets everything the hub still has) or a specific event ID (gets everything after it). The open-source hub has unlimited history by default; Cloud tiers cap it at 100-5,000 messages depending on plan. See [Reconnection and history](../concepts/reconnection-and-history.md).

## How do I get help?

- [GitHub Discussions](https://github.com/dunglas/mercure/discussions) for community questions.
- [Stack Overflow `mercure` tag](https://stackoverflow.com/questions/tagged/mercure).
- `#mercure` channel on the [Symfony Slack](https://symfony.com/slack).
- Cloud and Self-Hosted: [contact@mercure.rocks](mailto:contact@mercure.rocks). Self-Hosted Business / Corporate / Elite tiers include direct email or 24/7 support.
- [Les-Tilleuls.coop](https://les-tilleuls.coop/en/contact) provides commercial support and [official training](https://les-tilleuls.coop/en/masterclass/trainings/introduction-to-mercure) (English and French).
