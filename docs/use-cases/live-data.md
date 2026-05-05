---
title: "Real-Time Dashboards and Live Data Feeds with Mercure"
description: "Push live values, IoT telemetry, stock tickers, and dashboard updates to web and mobile clients using Mercure topics and URL Patterns."
---

# Live Data and Dashboards

The textbook Mercure use case: a value or set of values changes on the server, and connected clients see the change without polling. Stock tickers, room occupancy, IoT telemetry, sales counters, build statuses — anything where polling would either be too slow or too wasteful.

## The Shape of the Problem

```text
# The shape of the problem
   data source                hub                        clients
        │                      │                            │
        │  POST /publish       │   GET /sub?match=...       │
        │ ────────────────────►│ ───────────────────────────│
        │  (when value changes)│                            │
        │                      │ ───────────────────────────│  every connected
        │                      │ ───────────────────────────│  client gets it
```

Whatever produces the value (a database trigger, a webhook handler, an MQTT bridge, a worker reading from a queue) becomes the publisher. Browsers, mobile apps, and other servers subscribe.

## Topic Design

The natural topic for a data point is its URL — the same URL that returns its current value as JSON.

| Domain | Topic |
| --- | --- |
| Per-product availability | `https://shop.example.com/products/42/availability` |
| Per-room occupancy | `https://office.example.com/rooms/auditorium/occupancy` |
| Per-device telemetry | `urn:device:thermostat-01/temperature` |
| Build status by repo and branch | `https://ci.example.com/builds/owner/repo/main` |

When clients want a *family* of values, use `matchURLPattern`:

```javascript
// Topic design
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append("matchURLPattern", "https://shop.example.com/products/:id/availability");
new EventSource(url);
```

One connection, every product's availability changes flow over it.

## Subscriber

```html
<!-- Subscriber -->
<table id="prices">
  <tr data-topic="https://prices.example.com/AAPL"><td>AAPL</td><td class="value">--</td></tr>
  <tr data-topic="https://prices.example.com/GOOG"><td>GOOG</td><td class="value">--</td></tr>
</table>

<script>
  const url = new URL("https://hub.example.com/.well-known/mercure");
  url.searchParams.append("matchURLPattern", "https://prices.example.com/:symbol");

  const es = new EventSource(url);
  es.onmessage = (event) => {
    const update = JSON.parse(event.data);
    const cell = document.querySelector(
      `tr[data-topic="https://prices.example.com/${update.symbol}"] .value`
    );
    if (cell) cell.textContent = update.price;
  };
</script>
```

Two things to notice:

1. **Bootstrap from the origin first.** Render the page with the values you have; let Mercure deliver only the diffs. If you wait for the first SSE message before showing anything, your page is empty for as long as it takes for *any* value to change.
2. **Idempotent updates.** Each event carries the full new value. A reconnect or replay re-applies the same value harmlessly. For partial updates (JSON Patch), see [Reconnection and history](../concepts/reconnection-and-history.md#detecting-data-loss-in-mercure-replay).

## Publisher

A worker tailing a price feed:

```python
# Publisher
def on_price_change(symbol: str, price: float) -> None:
    requests.post(
        HUB,
        headers={"Authorization": f"Bearer {PUBLISHER_JWT}"},
        data={
            "topic": f"https://prices.example.com/{symbol}",
            "data": json.dumps({"symbol": symbol, "price": price, "ts": time.time()}),
        },
    )
```

Or, in your existing API service, fire the publish from the same code path that writes the database:

```python
# Publisher
def update_availability(product_id: int, in_stock: bool) -> None:
    db.update(product_id, in_stock=in_stock)
    publish(
        topic=f"https://shop.example.com/products/{product_id}/availability",
        data=json.dumps({"in_stock": in_stock}),
    )
```

For exactly-once-ish guarantees, write to a local outbox in the same transaction as the data change and have a worker drain it to the hub.

## Public vs. Private Mercure Live Data Topics

For data the whole world can see (public stock prices, public game scores), publish without `private=on`. Subscribers don't need a JWT.

For per-user or per-tenant data (a customer's order status, a tenant's CI runs), publish private and authorize the matchers in the subscriber's JWT. The [per-user authorization pattern](../concepts/authorization.md#per-user-authorization-on-shared-topics) covers shared-topic-with-fine-grained-access setups.

## Sizing the Mercure History Buffer for Live Data

For pure live data (the latest value is what matters; old values are useless), the history buffer doesn't have to be large. A handful of messages is enough to cover reconnects.

For replay-driven dashboards (replay the last hour of price changes when the page loads), you want a bigger buffer — or pair the hub with a primary store you can fetch from for the cold-start, and use Mercure only for incremental updates.

> **Pro tip.** The open-source hub stores history in BoltDB with **no built-in cap** — it grows until disk fills. Set `size N` in the transport config to bound it. Cloud tiers cap history at 100–5,000 messages depending on plan; if your dashboards need long histories and predictable storage, [Self-Hosted Mercure](https://mercure.rocks/pricing) with the Postgres transport keeps the data on infrastructure you control.

## Dashboards: Many Topics, One Connection

A dashboard that watches dozens of metrics opens **one** `EventSource` and uses many `match*` parameters, not one connection per metric:

```javascript
// Dashboards: many topics, one connection
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append("matchURLPattern", "https://metrics.example.com/cpu/:host");
url.searchParams.append("matchURLPattern", "https://metrics.example.com/memory/:host");
url.searchParams.append("matchURLPattern", "https://metrics.example.com/disk/:host/:device");
url.searchParams.append("matchRegexp", "^https://alerts.example.com/.+/firing$");

new EventSource(url);
```

The hub multiplexes them all over a single TCP connection. The browser's HTTP/2 stack does the rest.

## Mercure Throughput in Practice

Public benchmarks: a t3.micro running the open-source hub holds **40k concurrent SSE connections** with the BoltDB transport. Connections aren't expensive in Mercure — every additional client costs a goroutine and a few KB of RAM. Where you'll feel cost is publish throughput: the hub fans every update out to every matching subscriber, so high-rate streams to many subscribers means high outbound bandwidth.

For setups beyond what one node can handle, see [High availability](../production/high-availability.md).

## Next Steps for Mercure Live Data

- [Reconnection and history](../concepts/reconnection-and-history.md) — replay after a disconnect.
- [Authorization](../concepts/authorization.md) — per-user data.
- [Load testing](../production/load-testing.md) — figure out what your hardware can handle before users do.
