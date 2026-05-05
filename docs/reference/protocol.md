# Protocol

Mercure is a public protocol, not just an implementation. The canonical source of truth is the [IETF Internet-Draft](https://datatracker.ietf.org/doc/draft-dunglas-mercure/), on track for publication as an RFC. The full text is also kept in this repository:

- **[The Mercure Protocol specification](../../spec/mercure.md)**
- [OpenAPI definition](https://github.com/dunglas/mercure/blob/master/spec/openapi.yaml)

This page is a quick orientation, not a substitute. Read the spec for normative language and edge cases.

## What's in the protocol

- **Subscription** — `GET /.well-known/mercure?match=...` with one or more matcher query parameters.
- **Publication** — `POST /.well-known/mercure` with form-encoded `topic`, `data`, and friends.
- **Authorization** — JWT with a `mercure` claim that lists allowed matchers per role (publish, subscribe).
- **Reconnection** — `Last-Event-ID` header and `lastEventID` query parameter for replay.
- **Active subscriptions** — subscription events on a well-known topic family + a JSON-LD API.
- **Discovery** — `Link: rel="mercure"` headers on the publisher's resources.
- **Encryption** — JWE for end-to-end privacy.

## Matcher types (1.0)

| Matcher | Query parameter | Required of hubs | Reference |
| --- | --- | --- | --- |
| `Exact` | `match`, `matchExact` | **MUST** | spec §3.1 |
| `URLPattern` | `matchURLPattern` | **SHOULD** | [WHATWG URL Pattern](https://urlpattern.spec.whatwg.org) |
| `Regexp` | `matchRegexp` | **SHOULD** | [RFC 9485 (I-Regexp)](https://www.rfc-editor.org/rfc/rfc9485) |
| `CEL` | `matchCEL` | **MAY** | [Common Expression Language](https://cel.dev/) |
| `URITemplate` | `matchURITemplate` | **MAY** | [RFC 6570](https://www.rfc-editor.org/rfc/rfc6570) |

See [Topics and matchers](../concepts/topics-and-matchers.md) for the developer-facing tour.

## Implementations

- **[Mercure.rocks Hub](https://github.com/dunglas/mercure)** — the reference implementation. Caddy module, Go library, single static binary. Open-source (AGPL-3.0).
- **[Freddie](https://github.com/bpolaszek/freddie)** — PHP hub. Stable; covers everything except subscription events.
- **[Ilshidur/node-mercure](https://github.com/Ilshidur/node-mercure)** — Node.js hub and publisher. Beta.
- **[Symfony Mercure component](https://symfony.com/doc/current/mercure.html)** — PHP publisher and Symfony integration.
- **[API Platform](https://api-platform.com/docs/core/mercure/)** — full publisher + subscriber + GraphQL subscription support.
- **[Laravel Mercure Broadcaster](https://github.com/mvanduijker/laravel-mercure-broadcaster)** — publisher for Laravel.
- **[dart_mercure](https://github.com/wallforfry/dart_mercure)** — Dart / Flutter publisher and subscriber.

A non-exhaustive list — see [Awesome Mercure](../ecosystem/awesome.md) for client libraries in other languages and the full ecosystem.

## Conformance

The reference test suite is published in the [`conformance-tests/`](https://github.com/dunglas/mercure/tree/main/conformance-tests) directory of the repository. Any hub claiming to implement the spec should pass it. See [Conformance tests](../ecosystem/conformance-tests.md) for how to run the suite against your hub.

## Versioning

The protocol is versioned via the IETF draft number (currently `draft-dunglas-mercure-07`). The reference hub follows semver and ships breaking changes only at major versions. The current major is **1.0**, aligned with the typed-matcher model described in the spec.

If you're upgrading from a previous version, see the [upgrade guide](../UPGRADE.md).

## Patent and copyright

The specification is published under the [IETF copyright policy](https://trustee.ietf.org/copyright-faq.html). It can be implemented by any software, proprietary or otherwise. The reference hub itself is AGPL-3.0; see [License](license.md) for what that means in practice.
