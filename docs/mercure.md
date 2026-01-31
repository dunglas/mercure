# Mercure in a Few Words

Mercure is an open solution for real-time communications designed to be fast, reliable, and battery-efficient. It is a modern and convenient replacement for both the WebSocket API and the higher-level libraries and services that rely on it.

Mercure is especially useful for adding streaming and asynchronous capabilities to REST and GraphQL APIs. Because it is a thin layer on top of HTTP and SSE, Mercure is natively supported by modern web browsers, mobile applications, and IoT devices.

A free (as in beer and as in speech) reference server, a commercial High Availability version, and a hosted service are available.

![Subscriptions Schema](../spec/subscriptions.png)

## The Protocol

- Native browser support, no library or SDK required (built on top of HTTP and [server-sent events](https://www.smashingmagazine.com/2018/02/sse-websockets-data-flow-http2/))
- Compatible with all existing servers, even those that don't support persistent connections (serverless architecture, PHP, FastCGI, etc.)
- Built-in connection re-establishment and state reconciliation
- [JWT](https://jwt.io/)-based authorization mechanism (securely dispatch updates to selected subscribers)
- High performance, leverages [HTTP multiplexing](https://web.dev/performance-http2/#request-and-response-multiplexing)
- Designed with [hypermedia in mind](https://en.wikipedia.org/wiki/HATEOAS); also supports [GraphQL](https://graphql.org/)
- Auto-discoverable through [web linking](https://tools.ietf.org/html/rfc5988)
- Message encryption support
- Can work with old browsers (IE7+) using [an `EventSource` polyfill](ecosystem/awesome.md#useful-related-libraries)
- [Connection-less push](https://html.spec.whatwg.org/multipage/server-sent-events.html#eventsource-push) in controlled environments (e.g., browsers on mobile handsets tied to specific carriers)

[Read the specification](../spec/mercure.md)

## The Hub

- Fast, written in Go
- Works everywhere: [static binaries](hub/install.md#prebuilt-binary), [Docker image](hub/install.md#docker-image), and [Kubernetes chart](hub/install.md#kubernetes)
- Automatic HTTP/2, HTTP/3, and HTTPS (using Let's Encrypt) support
- [Clustering and High Availability support](hub/cluster.md)
- CORS support and CSRF protection mechanism
- Cloud Native, follows [the Twelve-Factor App](https://12factor.net) methodology
- Free and open source (AGPL), SaaS, and commercial versions available

[Get your hub](hub/install.md)
