---
title: "Mercure protocol conformance tests with Playwright"
description: "Validate any Mercure hub implementation against the Mercure protocol with the official Playwright-based conformance test suite."
---

# Mercure conformance tests

The Mercure repository ships a [Playwright](https://playwright.dev/)-based conformance test suite. It exercises the protocol against a running hub and checks that the responses match the spec.

Use it to:

- Validate a third-party Mercure implementation.
- Catch regressions when modifying the reference hub.
- Understand the protocol by reading concrete examples.

## Run the Mercure conformance test suite

```console
git clone https://github.com/dunglas/mercure
cd mercure/conformance-tests
npm ci
npx playwright install --with-deps
npx playwright test
```

The suite uses `https://localhost/.well-known/mercure` by default and runs browser code from the site root. Start an isolated [playground hub](../getting-started/quickstart.md) with the bundled development key before running it. The suite embeds a development token; changing the target also requires matching its issuer, audience, and signing key.

```console
BASE_URL=https://hub.example.com/.well-known/mercure npx playwright test
```

## Mercure conformance test configuration

| Variable    | Description                                                                |
| ----------- | -------------------------------------------------------------------------- |
| `BASE_URL`  | URL of the hub to test.                                                    |
| `CUSTOM_ID` | Toggle tests that depend on the hub honoring publisher-supplied event IDs. |

Set `CUSTOM_ID=1` to assert that the publish response preserves custom event IDs. Leave it unset to skip that assertion; the string `0` is truthy and enables it too.

## What the Mercure conformance suite covers

The current browser suite exercises publication, topic matching, and public/private delivery. It is not an exhaustive check of OAuth validation, replay, or the subscription API; those also have Go tests in this repository.

Run with `--ui` for the interactive Playwright explorer; useful when debugging a specific assertion failure.

## Related Mercure testing resources

- [Load test](../production/load-testing.md): measures throughput, not correctness.
- [Protocol](../reference/protocol.md): the spec the tests are validating against.
