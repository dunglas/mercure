# Metrics

The Mercure.rocks Hub relies on [Caddy's metrics](https://caddyserver.com/docs/metrics) and extends them with Mercure-specific collectors.

Enable Caddy's Prometheus scrape endpoint as described in the [Caddy documentation](https://caddyserver.com/docs/metrics). In addition to Caddy's HTTP, filesystem, and reverse-proxy metrics, the Hub registers the following collectors:

| Name                            | Type    | Description                                               |
| ------------------------------- | ------- | --------------------------------------------------------- |
| `mercure_subscribers_total`     | Counter | Total number of subscribers handled since the hub started |
| `mercure_subscribers_connected` | Gauge   | Current number of connected subscribers                   |
| `mercure_updates_total`         | Counter | Total number of updates dispatched since the hub started  |
