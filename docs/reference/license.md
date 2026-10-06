---
title: "Mercure.rocks hub license: AGPL-3.0 and commercial options"
description: "Licensing for the Mercure protocol (open) and the Mercure.rocks Hub (AGPL-3.0), plus commercial Self-Hosted licensing options."
---

# Mercure license

Short version:

- The **Mercure protocol** is open. Anyone can implement it, including in proprietary software.
- The **Mercure.rocks Hub** is licensed under [AGPL-3.0](../../LICENSE). If you distribute a modified hub or let users interact with it over a network, you must offer the corresponding source under the license's terms.
- **Independent applications that use the hub over HTTP** can keep their own license: proprietary, MIT, GPL, or another license. Publishing and subscribing do not make your application AGPL.
- For organizations that need different terms, **[commercial licenses are available](https://mercure.rocks/pricing)**.

## The Mercure protocol license

The [Mercure specification](../../spec/mercure.md) is published with the IETF's copyright and legal notices. You can implement the protocol independently, including in proprietary software. The hub's AGPL license applies to its implementation, not to the protocol itself.

## The Mercure.rocks reference hub license

The Mercure.rocks Hub (this repository) is [AGPL-3.0](../../LICENSE).

In practice:

- **Running the hub.** You can use it for personal or commercial projects.
- **Modifying the hub.** If you convey a modified version, or users interact with it remotely over a network, you must provide its Corresponding Source as required by the AGPL.
- **Publishers and subscribers.** A proprietary backend that publishes over HTTP to a Mercure hub stays proprietary. The same applies to independent subscriber applications. Embedding or linking the hub into another program is a different case.

## Commercial licenses

**[Mercure Enterprise, available through Self-Hosted plans](https://mercure.rocks/pricing), includes a commercial license, multi-node transports, and direct support.** Run the hub on your infrastructure with Redis/Valkey, PostgreSQL, Kafka, or Pulsar. [Compare plans](https://mercure.rocks/pricing) for current prices and support terms.

For custom licensing, contact [contact@mercure.rocks](mailto:contact@mercure.rocks). Your legal team can review the commercial terms before purchase.

Prefer managed hosting? **[Mercure Cloud](https://mercure.rocks/pricing)** lets you use Mercure without installing or maintaining a hub.

## Mercure trademarks

"Mercure" and "Mercure.rocks" are trademarks of Dunglas Services SAS. The AGPL does not license these trademarks. Contact [contact@mercure.rocks](mailto:contact@mercure.rocks) for permission to use them in your product's name or branding. Compatible implementations may describe themselves as "compatible with the Mercure protocol."

## Contributing to the Mercure.rocks hub

Contributions are welcome under AGPL-3.0. By submitting a pull request you agree to license your contribution under the same terms as the rest of the project. See [CONTRIBUTING.md](https://github.com/dunglas/mercure/blob/main/CONTRIBUTING.md) for the development workflow.

## Mercure patent grant

The AGPL-3.0 includes an explicit patent grant from contributors. The protocol itself, published as an IETF Internet-Draft, is subject to the IETF's patent policy.

## Mercure licensing questions

For licensing questions that aren't answered here, email [contact@mercure.rocks](mailto:contact@mercure.rocks).
