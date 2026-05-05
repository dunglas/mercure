# License

Short version:

- The **Mercure protocol** is open. Anyone can implement it, including in proprietary software.
- The **Mercure.rocks Hub** is licensed under [AGPL-3.0](https://github.com/dunglas/mercure/blob/master/LICENSE). Modifications to the hub itself must be shared.
- **Software that uses the hub** (publishers, subscribers, your application) is **not affected** by the AGPL. Use any license you want — proprietary, MIT, GPL, anything.
- For organizations that can't use AGPL software, [commercial licenses are available](https://mercure.rocks/pricing).

## The protocol

The [Mercure specification](../../spec/mercure.md) is published under the [IETF copyright policy](https://trustee.ietf.org/copyright-faq.html). It can be implemented by any software, including proprietary software, with no royalty or attribution requirement.

## The reference hub

The Mercure.rocks Hub (this repository) is [AGPL-3.0](https://github.com/dunglas/mercure/blob/master/LICENSE).

In practice:

- **Running the hub.** No restriction on who runs it or for what purpose.
- **Modifying the hub.** Your modifications must be made available under AGPL-3.0 to anyone you give the modified version to — including users who interact with it over a network.
- **Publishers and subscribers.** Software that talks to the hub over the protocol does **not** become AGPL. The AGPL is about the hub binary itself, not anything that connects to it. A proprietary backend that publishes to a Mercure hub stays proprietary.

This is the same shape of license as MongoDB Community pre-SSPL: it lets anyone use the hub freely while preventing third parties from running modified versions as a proprietary SaaS without contributing back.

## Commercial licenses

If your organization can't use AGPL-3.0 (some compliance teams refuse it categorically), commercial licenses are available:

- **[Self-Hosted Mercure](https://mercure.rocks/pricing)** — comes with a commercial license to the binary, plus the multi-node transports and direct support. Pricing starts at €1,500/year.
- **Custom licensing** for cases that don't fit either option — [contact@mercure.rocks](mailto:contact@mercure.rocks).

The commercial license is grant-based and doesn't require contributing modifications back. Your legal team can review the terms before purchase.

## Trademarks

"Mercure" and "Mercure.rocks" are trademarks of Dunglas Services SAS. The trademarks are not licensed under the AGPL — you can't use them in the name of your own product without permission. Implementations that pass [the conformance tests](../ecosystem/conformance-tests.md) may describe themselves as "compatible with the Mercure protocol."

## Contributing

Contributions are welcome under AGPL-3.0. By submitting a pull request you agree to license your contribution under the same terms as the rest of the project. See [CONTRIBUTING.md](https://github.com/dunglas/mercure/blob/main/CONTRIBUTING.md) for the development workflow.

## Patent grant

The AGPL-3.0 includes an explicit patent grant from contributors. The protocol itself, published as an IETF Internet-Draft, is subject to the IETF's patent policy.

## Questions

For licensing questions that aren't answered here, email [contact@mercure.rocks](mailto:contact@mercure.rocks).
