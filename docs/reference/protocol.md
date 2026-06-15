---
title: "Mercure protocol specification overview"
description: "Introduction to the Mercure IETF specification: subscriptions, publications, JWT authorization, replay, active subscriptions, and matcher types."
---

# Protocol

Mercure is a public protocol, not just an implementation. The canonical source of truth is the [IETF Internet-Draft](https://datatracker.ietf.org/doc/draft-dunglas-mercure/), on track for publication as an RFC. The full text is also kept in this repository:

- **[The Mercure Protocol specification](../../spec/mercure.md)**
- [OpenAPI definition](https://github.com/dunglas/mercure/blob/master/spec/openapi.yaml)

This page is a quick orientation, not a substitute. Read the spec for normative language and edge cases.

## What's in the protocol

- **Subscription**: `GET /.well-known/mercure?match=...` with one or more matcher query parameters.
- **Publication**: `POST /.well-known/mercure` with form-encoded `topic`, `data`, and friends.
- **Authorization**: OAuth 2.0 JWT access tokens ([RFC 9068](https://www.rfc-editor.org/rfc/rfc9068)) with an `authorization_details` claim ([RFC 9396](https://www.rfc-editor.org/rfc/rfc9396)) granting publish/subscribe per topic matcher; [RFC 6750](https://www.rfc-editor.org/rfc/rfc6750) errors.
- **Reconnection**: `Last-Event-ID` header and `lastEventID` query parameter for replay.
- **Active subscriptions**: subscription events on a well-known topic family + a JSON-LD API.
- **Discovery**: `Link: rel="mercure"` headers on the publisher's resources, plus OAuth 2.0 protected resource metadata ([RFC 9728](https://www.rfc-editor.org/rfc/rfc9728)).
- **Encryption**: JWE for end-to-end privacy.

## Matcher types

| Matcher      | Query parameter       | Required of hubs | Reference                                                |
| ------------ | --------------------- | ---------------- | -------------------------------------------------------- |
| `Exact`      | `match`, `matchExact` | **MUST**         | exact string comparison                                  |
| `URLPattern` | `matchURLPattern`     | **MUST**         | [WHATWG URL Pattern](https://urlpattern.spec.whatwg.org) |

See [Topics and matchers](../concepts/topics-and-matchers.md) for the developer-facing tour.

## Mercure protocol implementations

- **[Mercure.rocks Hub](https://github.com/dunglas/mercure)**: the reference implementation. Caddy module, Go library, single static binary. Open-source (AGPL-3.0).
- **[Freddie](https://github.com/bpolaszek/freddie)**: PHP hub. Stable; covers everything except subscription events.
- **[Ilshidur/node-mercure](https://github.com/Ilshidur/node-mercure)**: Node.js hub and publisher. Beta.
- **[Symfony Mercure component](https://symfony.com/doc/current/mercure.html)**: PHP publisher and Symfony integration.
- **[API Platform](https://api-platform.com/docs/core/mercure/)**: full publisher + subscriber + GraphQL subscription support.
- **[Laravel Mercure Broadcaster](https://github.com/mvanduijker/laravel-mercure-broadcaster)**: publisher for Laravel.
- **[dart_mercure](https://github.com/wallforfry/dart_mercure)**: Dart / Flutter publisher and subscriber.

A non-exhaustive list, see [Awesome Mercure](../ecosystem/awesome.md) for client libraries in other languages and the full ecosystem.

## Mercure protocol conformance

The reference test suite is published in the [`conformance-tests/`](https://github.com/dunglas/mercure/tree/main/conformance-tests) directory of the repository. Any hub claiming to implement the spec should pass it. See [Conformance tests](../ecosystem/conformance-tests.md) for how to run the suite against your hub.

## Mercure protocol versioning

The protocol is versioned via the IETF draft number. The reference hub follows semver and ships breaking changes only at major versions. The current major is **1.0**, aligned with the two-matcher, OAuth 2.0 authorization model described in the spec.

If you're upgrading from a previous version, see the [upgrade guide](../UPGRADE.md).

## Mercure protocol patent and copyright

The specification is published under the [IETF copyright policy](https://trustee.ietf.org/copyright-faq.html). It can be implemented by any software, proprietary or otherwise. The reference hub itself is AGPL-3.0; see [License](license.md) for what that means in practice.
