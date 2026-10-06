---
title: "Real-time dashboards and live data feeds with Mercure"
description: "Push live values, IoT telemetry, stock tickers, and dashboard updates to web and mobile clients using Mercure topics and URL Patterns."
---

# Mercure live data and dashboards

Publish changing values to Mercure so connected dashboards can update without polling. Examples include stock prices, room occupancy, device telemetry, and build status.

## The shape of the problem

```text
   data source                hub                        clients
        |                      |                            |
        |  POST to hub       |   GET hub?match=...       |
        | -------------------> | -------------------------> |
        |  (when value changes)|                            |
        |                      | -------------------------> |  every connected
        |                      | -------------------------> |  client gets it
```

Whatever produces the value (a database trigger, a webhook handler, an MQTT bridge, a worker reading from a queue) becomes the publisher. Browsers, mobile apps, and other servers subscribe.

## Topic design

The natural topic for a data point is its URL, the same URL that returns its current value as JSON.

| Domain                                | Topic                                                   |
| ------------------------------------- | ------------------------------------------------------- |
| Per-product availability              | `https://shop.example.com/products/42/availability`     |
| Per-room occupancy                    | `https://office.example.com/rooms/auditorium/occupancy` |
| Per-device telemetry                  | `urn:device:thermostat-01/temperature`                  |
| Build status by repository and branch | `https://ci.example.com/builds/owner/repo/main`         |

When clients want a _family_ of values, use `match_urlpattern`:

```javascript
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append(
  "match_urlpattern",
  "https://shop.example.com/products/:id/availability",
);
new EventSource(url);
```

One connection, every product's availability changes flow over it.

## Subscriber

```html
<table id="prices">
  <tr data-topic="https://prices.example.com/AAPL">
    <td>AAPL</td>
    <td class="value">--</td>
  </tr>
  <tr data-topic="https://prices.example.com/GOOG">
    <td>GOOG</td>
    <td class="value">--</td>
  </tr>
</table>

<script>
  const url = new URL("https://hub.example.com/.well-known/mercure");
  url.searchParams.append(
    "match_urlpattern",
    "https://prices.example.com/:symbol",
  );

  const es = new EventSource(url);
  es.onmessage = (event) => {
    const update = JSON.parse(event.data);
    const cell = document.querySelector(
      `tr[data-topic="https://prices.example.com/${update.symbol}"] .value`,
    );
    if (cell) cell.textContent = update.price;
  };
</script>
```

Two things to notice:

1. **Bootstrap from the origin first.** Render the page with the values you have; let Mercure deliver only the diffs. If you wait for the first SSE message before showing anything, your page is empty for as long as it takes for _any_ value to change.
2. **Idempotent updates.** Each event carries the full new value. A reconnect or replay re-applies the same value harmlessly. For partial updates (JSON Patch), see [Reconnection and history](../concepts/reconnection-and-history.md#detecting-data-loss-in-mercure-replay).

## Publisher

This worker sketch uses `requests`, `json`, and `time`. Configure `HUB` and `PUBLISHER_JWT` for your deployment:

```python
import json
import time
import requests

def on_price_change(symbol: str, price: float) -> None:
    response = requests.post(
        HUB,
        headers={"Authorization": f"Bearer {PUBLISHER_JWT}"},
        data={
            "topic": f"https://prices.example.com/{symbol}",
            "data": json.dumps({"symbol": symbol, "price": price, "ts": time.time()}),
        },
        timeout=10,
    )
    response.raise_for_status()
```

Or, in your existing API service, fire the publish from the same code path that writes the database:

```python
def update_availability(product_id: int, in_stock: bool) -> None:
    db.update(product_id, in_stock=in_stock)
    publish(
        topic=f"https://shop.example.com/products/{product_id}/availability",
        data=json.dumps({"in_stock": in_stock}),
    )
```

For retryable publication, write an outbox entry in the same transaction as the data change. A worker publishes it and retries failures. Consumers must handle duplicates.

## Public vs. Private Mercure live data topics

For public data, omit `private=on`. Enable `anonymous` if subscribers should connect without a token.

For per-user or per-tenant data (a customer's order status, a tenant's CI runs), publish private and authorize the matchers in the subscriber's access token. The [per-user authorization pattern](../concepts/authorization.md#per-user-authorization-on-shared-resources) covers per-resource fine-grained access setups.

## Sizing the Mercure history buffer for live data

Size retention from the total event rate and expected disconnect duration. For latest-value dashboards, clients can fetch a fresh snapshot instead of replaying a long history.

For replay-driven dashboards (replay the last hour of price changes when the page loads), you want a bigger buffer, or pair the hub with a primary store you can fetch from for the cold-start, and use Mercure only for incremental updates.

For shared dashboard history on your own infrastructure, [Mercure Enterprise with PostgreSQL](../production/high-availability.md#postgresql) keeps events in a database you control. Prefer managed hosting? [Mercure Cloud](https://mercure.rocks/pricing) operates the hub for you.

## Dashboards: many topics, one connection

A dashboard that watches dozens of metrics opens **one** `EventSource` and uses many `match*` parameters, not one connection per metric:

```javascript
const url = new URL("https://hub.example.com/.well-known/mercure");
url.searchParams.append(
  "match_urlpattern",
  "https://metrics.example.com/cpu/:host",
);
url.searchParams.append(
  "match_urlpattern",
  "https://metrics.example.com/memory/:host",
);
url.searchParams.append(
  "match_urlpattern",
  "https://metrics.example.com/disk/:host/:device",
);
url.searchParams.append(
  "match_urlpattern",
  "https://alerts.example.com/:service/firing",
);

new EventSource(url);
```

One SSE stream delivers all matching updates.

## Mercure throughput in practice

A [published benchmark](https://speakerdeck.com/dunglas/2-plus-and-mercure?slide=41) reported **40,000 concurrent connections on an EC2 t3.micro** with the open-source hub. For a large live audience, [Mercure Enterprise](../production/high-availability.md) lets you distribute subscribers across several nodes.

Fan-out determines outbound traffic: each update is sent to every matching subscriber. Measure your payload sizes, publish rate, subscriber counts, and history writes using the [load test](../production/load-testing.md).

For setups beyond what one node can handle, see [High availability](../production/high-availability.md).

## Next steps

- [Reconnection and history](../concepts/reconnection-and-history.md): replay after a disconnect.
- [Authorization](../concepts/authorization.md): per-user data.
- [Load testing](../production/load-testing.md): figure out what your hardware can handle before users do.
