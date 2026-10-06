---
title: "Mercure FAQ: WebSockets, Pusher, GraphQL, and connection limits"
description: "Common Mercure questions: comparisons with WebSockets, Pusher, Ably, Firebase, Supabase Realtime, WebSub, and Web Push, and answers about connection limits and history."
---

# Mercure FAQ

## What's the difference between Mercure and WebSockets?

Mercure gives you the features you'd otherwise build around a WebSocket connection: topic authorization, automatic reconnection, replay, and presence. It uses ordinary HTTP requests and Server-Sent Events, with native browser support and HTTP/2 or HTTP/3 multiplexing.

For notifications, live dashboards, AI streaming, and collaborative apps, Mercure lets you spend less time building messaging infrastructure. Client actions go through your existing HTTP API; the hub pushes the results to connected users. [Mercure Cloud](https://mercure.rocks/pricing) handles the hub for you.

WebSockets remain useful when an application needs frequent, bidirectional messages on one dedicated channel. See the [use-case guides](../use-cases/README.md) for Mercure patterns.

## What's the difference between Mercure and Pusher / Ably / Firebase / Supabase Realtime?

**Mercure gives you a choice of hosting without tying your application to a proprietary client SDK.** Publish with any HTTP client, subscribe with the browser's native `EventSource`, and keep your existing database and backend.

- **Moving from Pusher or Ably?** Mercure provides managed real-time delivery through [Mercure Cloud](https://mercure.rocks/pricing), with an open protocol and an [Enterprise on-premises option](../production/high-availability.md). Laravel applications can keep their broadcasting events and Echo API; see [Laravel broadcasting](../use-cases/laravel-broadcasting.md).
- **Evaluating Firebase or Supabase Realtime?** Mercure adds live updates to the stack you already use. You don't need to move application data into a new database service to send notifications, stream AI output, or update a dashboard.
- **Want control over infrastructure and cost?** The free open-source hub has no licensed connection or publish-rate cap. Enterprise adds shared transports and direct support for clusters on your own servers. Capacity still depends on hardware and configuration.

**[Choose Mercure Cloud](https://mercure.rocks/pricing) for managed hosting, or [Self-Hosted Enterprise](https://mercure.rocks/pricing) for control over your deployment.** Both speak the same protocol as the open-source hub.

## What's the difference between Mercure and WebSub?

[WebSub](https://www.w3.org/TR/websub/) delivers updates to server callbacks. Mercure uses SSE, which browsers can consume directly, and adds topic authorization and replay.

## What's the difference between Mercure and Web Push?

[Web Push](https://www.w3.org/TR/push-api/) can reach a service worker when the application page is closed, subject to browser and platform policies. Mercure delivers updates over an active subscription.

Use Mercure for updates inside an open application. Use Web Push when you need to notify users outside it. Mercure replay is limited by retained history.

## How many connections can a single hub hold?

Published results include **40,000 concurrent connections on an EC2 t3.micro** with the open-source hub and **200,000 with the on-premises HA version**. See the [benchmark presentation](https://speakerdeck.com/dunglas/2-plus-and-mercure?slide=41).

The open-source hub has no licensed connection cap. Your capacity depends on publication rate, payload size, fan-out, and hardware; run the [load test](../production/load-testing.md) with representative traffic. For redundancy or more capacity, [Mercure Enterprise](../production/high-availability.md) distributes subscribers across nodes. [Mercure Cloud](https://mercure.rocks/pricing) lets you scale without operating them.

## What's the maximum number of open connections per browser?

With HTTP/2, **100 concurrent streams is a common default**, negotiated between the browser and server. These streams share a connection. HTTP/1.1 typically allows **6 connections per origin**, shared across tabs. See [MDN's SSE connection limits](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#receiving_events_from_the_server). HTTP/3 also multiplexes streams with negotiated limits.

Use several matchers on one `EventSource` when possible. This hub accepts up to 100 matchers per subscription; a URL Pattern can select many topics.

## Can a single subscriber be in many topics over one connection?

Yes. Pass several `match*` parameters:

```javascript
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append("match", "https://example.com/announcements");
url.searchParams.append(
  "match_urlpattern",
  "https://example.com/users/:id/notifications",
);
new EventSource(url);
```

One SSE stream carries updates matching either parameter.

## How do I use Mercure with GraphQL?

Mercure is delivery-agnostic, so it pairs cleanly with GraphQL subscriptions: the GraphQL integration returns a topic through an application-defined registration request, the client opens an `EventSource` on that topic. See [GraphQL subscriptions](../use-cases/graphql.md). The [API Platform framework](https://api-platform.com/docs/master/core/graphql/#subscriptions) ships this integration out of the box.

## How do I use Mercure with Hotwire / Turbo Streams?

Connect a Turbo Stream source to an `EventSource`:

```javascript
import { connectStreamSource } from "@hotwired/turbo";
connectStreamSource(new EventSource(mercureUrl));
```

Publish HTML with `<turbo-stream>` tags as the `data` field. See [Hotwire / Turbo Streams](../use-cases/hotwire.md).

## How do I send the authorization cookie to the hub?

For cross-origin `EventSource`, set `withCredentials`:

```javascript
new EventSource(url, { withCredentials: true });
```

The hub must allow the application origin in its CORS headers (`cors_origins` listing the calling origin, no wildcard). See [Authorization](../concepts/authorization.md#cookies-in-detail).

## Can I run Mercure without a hub?

An application can implement the hub role itself or embed the [Go library](https://pkg.go.dev/github.com/dunglas/mercure). The standalone hub is useful when you want to manage subscriber connections separately from your application workers.

## Does Mercure work with serverless?

Yes, on the publisher side: a Lambda or Cloud Function can `POST` to the hub and exit. On the subscriber side, the hub is the long-lived process; your serverless functions don't have to keep connections open.

If you're thinking about running the hub itself on serverless: the hub holds long-lived connections, so platforms with execution timeouts (most serverless) don't fit. Run the hub somewhere with persistent compute.

## What's the Mercure hub delivery latency?

Latency depends on the network, transport, payload size, and subscriber fan-out. Measure from publication to receipt with your deployment. A successful publish response does not acknowledge processing by every subscriber.

## How do I monitor it?

Prometheus metrics on the admin port, plus the `mercure_subscribers_connected` and `mercure_updates_total` series for the most useful signals. Full list in [Health monitoring](../production/health-monitoring.md).

## What about messages I publish before any subscriber connects?

With a history-enabled transport, clients can request retained events with `last_event_id=earliest`, or resume after a specific event ID. BoltDB retains events without an automatic size cap by default, but searches for a specific ID are bounded. The local transport stores no history. See [Reconnection and history](../concepts/reconnection-and-history.md).

## How do I get help?

- [GitHub Discussions](https://github.com/dunglas/mercure/discussions) for community questions.
- [Stack Overflow `mercure` tag](https://stackoverflow.com/questions/tagged/mercure).
- `#mercure` channel on the [Symfony Slack](https://symfony.com/slack).
- Cloud and Self-Hosted: [contact@mercure.rocks](mailto:contact@mercure.rocks). Self-Hosted Business / Corporate / Elite tiers include direct email or 24/7 support.
- [Les-Tilleuls.coop](https://les-tilleuls.coop/en/contact) provides commercial support and [official training](https://les-tilleuls.coop/en/masterclass/trainings/introduction-to-mercure) (English and French).
