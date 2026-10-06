---
title: "Mercure health checks, Prometheus metrics, and monitoring"
description: "Probe the Mercure transport-aware health endpoints, scrape Prometheus metrics, and monitor connection counts and dispatch failures."
---

# Mercure health checks and monitoring

Use the transport readiness endpoint to decide whether a hub can receive traffic. Use liveness to decide whether it needs a restart. A running process or an open HTTP port alone does not establish transport health.

## Mercure hub health endpoints

All health endpoints live on the Caddy admin API (default `localhost:2019`):

| Endpoint                           | Returns                                                                                                                       |
| ---------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `GET /mercure/health/ready`        | `200` if all transports can serve traffic, `503` otherwise. **Use for readiness.**                                            |
| `GET /mercure/health/live`         | `200` if all transports are fundamentally operational. `503` if a transport reports a liveness failure. **Use for liveness.** |
| `GET /mercure/health/{name}/ready` | Per-hub readiness when running multiple hubs.                                                                                 |
| `GET /mercure/health/{name}/live`  | Per-hub liveness.                                                                                                             |

The bundled BoltDB and local transports do not implement remote dependency checks, so their probes return `200`. This does not verify disk capacity or end-to-end delivery. Shared transports can implement their own readiness and liveness checks.

## Why Mercure has two health endpoints

Readiness can fail during a temporary transport outage. Liveness lets a transport distinguish temporary failures from conditions that require a restart; the exact checks depend on the transport.

Restarting a hub during a temporary backend outage also disconnects its subscribers. Configure restart thresholds to allow the transport time to recover.

## Probing from outside the container

The admin API binds to `localhost:2019` for security. That means standard `httpGet` probes, which run from outside the container, can't reach it. Use `exec` probes instead:

```yaml
readinessProbe:
  exec:
    command:
      [
        "wget",
        "-q",
        "-O",
        "/dev/null",
        "http://localhost:2019/mercure/health/ready",
      ]
  initialDelaySeconds: 10
  periodSeconds: 10
livenessProbe:
  exec:
    command:
      [
        "wget",
        "-q",
        "-O",
        "/dev/null",
        "http://localhost:2019/mercure/health/live",
      ]
  initialDelaySeconds: 30
  periodSeconds: 30
```

In Docker Compose:

```yaml
healthcheck:
  test:
    [
      "CMD",
      "wget",
      "-q",
      "-O",
      "/dev/null",
      "http://localhost:2019/mercure/health/ready",
    ]
  timeout: 5s
  retries: 5
  start_period: 60s
```

The 60s `start_period` matters: BoltDB takes a moment to open on first boot, so the first probe might fail; treat that as "not unhealthy yet."

If you absolutely need `httpGet` probes, you can bind the admin API to all interfaces:

```caddyfile
{
  admin 0.0.0.0:2019
}
```

This exposes the full admin API, including `/stop`, `/load`, and `/config`, to the pod network. Restrict access if you use this configuration.

## The legacy `/healthz` endpoint

The bundled Caddyfile no longer defines `/healthz`. Replace old probes with `/mercure/health/ready` and `/mercure/health/live` on the admin API. A manually configured `respond /healthz 200` checks only the HTTP route.

## Prometheus metrics

Enable metrics in `GLOBAL_OPTIONS`:

```caddyfile
{
  metrics
}
```

Metrics live on the admin API at `/metrics`. The hub exposes Caddy's built-in metrics plus Mercure-specific ones:

| Metric                          | Description                              |
| ------------------------------- | ---------------------------------------- |
| `mercure_subscribers_connected` | Current number of connected subscribers. |
| `mercure_subscribers_total`     | Total subscribers seen.                  |
| `mercure_updates_total`         | Total updates published.                 |

Plus standard Caddy metrics: request counts, latencies, in-flight requests, certificate expiry. See the [Caddy metrics docs](https://caddyserver.com/docs/metrics).

## Useful alerts for the Mercure hub

Set thresholds from your workload and availability targets:

| Alert               | Condition                                                                                          |
| ------------------- | -------------------------------------------------------------------------------------------------- |
| Hub down            | Prometheus `up` is `0` for the configured alert window.                                            |
| Reconnect storm     | `rate(mercure_subscribers_total[5m])` > 10x steady state.                                          |
| Transport unhealthy | Readiness endpoint returning 503.                                                                  |
| Slow dispatch       | Publish (`POST`) request duration p99 exceeds your target; exclude long-lived `GET` subscriptions. |
| Cert expiry         | Less than 14 days.                                                                                 |

## Mercure Grafana dashboards

A reasonable Grafana panel set:

- **Connections**: `mercure_subscribers_connected` per pod, stacked.
- **Publish rate**: `rate(mercure_updates_total[1m])`, with publish error responses from HTTP metrics or logs overlaid.
- **Reconnect rate**: `rate(mercure_subscribers_total[1m])`. Compare spikes with deployment and proxy events.
- **Transport health**: readiness endpoint state (a synthetic probe writing to a metric).
- **Latency**: request duration histograms from Caddy.

## Application-level Mercure health canaries

A canary should subscribe, publish a unique value, and confirm receipt before a deadline. Use a dedicated topic and a token with both publish and subscribe grants:

```bash
#!/bin/sh
set -eu
: "${HUB:?Set the HTTPS hub URL}"
: "${JWT:?Set a token authorized for the canary topic}"
TOPIC="https://example.com/_canary"
VALUE="$(date +%s)-$$"
OUTPUT=$(mktemp)

curl --silent --show-error --no-buffer --get "$HUB" \
  -H "Authorization: Bearer $JWT" \
  --data-urlencode "match=$TOPIC" \
  --data-urlencode 'last_event_id=earliest' \
  --max-time 10 > "$OUTPUT" &
SUBSCRIBER_PID=$!

curl --fail-with-body --silent --show-error "$HUB" \
  -H "Authorization: Bearer $JWT" \
  --data-urlencode "topic=$TOPIC" \
  --data-urlencode 'private=on' \
  --data-urlencode "data=$VALUE" > /dev/null

# curl times out because a healthy SSE stream stays open.
wait "$SUBSCRIBER_PID" || true
grep -Fq "data: $VALUE" "$OUTPUT" || { echo "canary failed"; exit 1; }
echo "canary ok (capture: $OUTPUT)"
```

This example requires a history-enabled transport so it also works if the publish arrives before the subscription opens. Limit retention on a dedicated canary hub; repeatedly replaying a large shared history is expensive.

Run it outside the cluster to test routing, TLS, publishing, and delivery. Test CORS separately in a browser.

## Mercure hub logging

Caddy and Mercure emit structured logs, normally on stderr. Useful fields include `level`, `msg`, `subscriber`, `update`, and `error`.

For deeper diagnostics, set `GLOBAL_OPTIONS=debug`. **Don't leave it on in production:** it logs full update payloads, which means private data ends up in your log pipeline.

## Mercure hub performance baselines

Measure memory, CPU, connection count, publication rate, and payload size under representative load. Fan-out and history writes affect capacity. Compare profiles at similar traffic levels before attributing memory growth to a leak.

## Next steps

- [Debugging](debugging.md): when metrics aren't enough.
- [Load testing](load-testing.md): establish a baseline before going live.
- [Rolling updates](rolling-updates.md): what your `mercure_subscribers_connected` chart should look like during a deploy.
