---
title: "Mercure Protocol Conformance Tests with Playwright"
description: "Validate any Mercure hub implementation against the Mercure protocol with the official Playwright-based conformance test suite."
---

# Conformance Tests

The Mercure repository ships a [Playwright](https://playwright.dev/)-based conformance test suite. It exercises the protocol against a running hub and checks that the responses match the spec.

Use it to:

- Validate a third-party Mercure implementation.
- Catch regressions when modifying the reference hub.
- Understand the protocol by reading concrete examples.

## Run the Mercure Conformance Test Suite

```console
# Run the Mercure Conformance Test Suite
git clone https://github.com/dunglas/mercure
cd mercure/conformance-tests
npm ci
npx playwright install --with-deps
npx playwright test
```

By default the suite hits a hub on `https://localhost`. Start one before running tests, or override `BASE_URL`:

```console
# Run the Mercure Conformance Test Suite
BASE_URL=https://hub.example.com npx playwright test
```

## Mercure Conformance Test Configuration

| Variable | Description |
| --- | --- |
| `BASE_URL` | URL of the hub to test. |
| `CUSTOM_ID` | Toggle tests that depend on the hub honoring publisher-supplied event IDs. |

Set `CUSTOM_ID=0` for transports that don't support custom IDs (e.g. Pulsar — see [High availability](../production/high-availability.md#self-hosted-transports) for transport feature matrices).

## What the Mercure Conformance Suite Covers

Tests are organized by spec section:

- Subscribe semantics (`match` and the typed matcher parameters).
- Publish semantics (form fields, alternate topics, custom IDs).
- Authorization (publisher and subscriber JWT validation).
- Reconnection (`Last-Event-ID`, `lastEventID`, `earliest`).
- Active subscriptions (events + API).

Run with `--ui` for the interactive Playwright explorer; useful when debugging a specific assertion failure.

## Related Mercure Testing Resources

- [Load test](../production/load-testing.md) — measures throughput, not correctness.
- [Protocol](../reference/protocol.md) — the spec the tests are validating against.
