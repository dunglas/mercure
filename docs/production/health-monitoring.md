---
title: "Mercure health checks, prometheus metrics, and monitoring"
description: "Probe the Mercure transport-aware health endpoints, scrape Prometheus metrics, and monitor connection counts and dispatch failures."
---

# Health checks and monitoring

What "the hub is healthy" means depends on the level you're checking at:

- **Process is running.** Useful for restart-on-crash. Trivial.
- **HTTP listener answers.** Useful for L4/L7 load balancers.
- **Transport is connected and ready to dispatch.** What you actually want for `readiness` probes.

The hub exposes the third level explicitly. Use it.

## Mercure hub health endpoints

All health endpoints live on the Caddy admin API (default `localhost:2019`):

| Endpoint                           | Returns                                                                                                                              |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| `GET /mercure/health/ready`        | `200` if all transports can serve traffic, `503` otherwise. **Use for readiness.**                                                   |
| `GET /mercure/health/live`         | `200` if all transports are fundamentally operational. `503` if any has been unhealthy for an extended period. **Use for liveness.** |
| `GET /mercure/health/{name}/ready` | Per-hub readiness when running multiple hubs.                                                                                        |
| `GET /mercure/health/{name}/live`  | Per-hub liveness.                                                                                                                    |

Bolt and local transports always return `200`: there's no remote system whose connection can fail. Redis, Postgres, Kafka, and Pulsar transports actively check the connection.

## Why Mercure has two health endpoints

Readiness should fail **fast**: a momentary blip on Redis, a Postgres failover, a Kafka rebalance, the pod isn't able to serve right now and traffic should route elsewhere. Liveness should fail **slow**: only when the pod is unrecoverable should the orchestrator restart it.

Restarting a hub on a transient transport blip just adds a reconnect storm to whatever the transport problem already was. The two endpoints encode that distinction.

## Probing from outside the container

The admin API binds to `localhost:2019` for security. That means standard `httpGet` probes, which run from outside the container, can't reach it. Use `exec` probes instead:

```yaml
# Probing from outside the container
readinessProbe:
  exec:
    command:
      ["wget", "-q", "--spider", "http://localhost:2019/mercure/health/ready"]
  initialDelaySeconds: 10
  periodSeconds: 10
livenessProbe:
  exec:
    command:
      ["wget", "-q", "--spider", "http://localhost:2019/mercure/health/live"]
  initialDelaySeconds: 30
  periodSeconds: 30
```

Same shape in Docker Compose:

```yaml
# Probing from outside the container
healthcheck:
  test:
    [
      "CMD",
      "wget",
      "-q",
      "--spider",
      "http://localhost:2019/mercure/health/ready",
    ]
  timeout: 5s
  retries: 5
  start_period: 60s
```

The 60s `start_period` matters: BoltDB takes a moment to open on first boot, so the first probe might fail; treat that as "not unhealthy yet."

If you absolutely need `httpGet` probes, you can bind the admin API to all interfaces:

```caddyfile
# Probing from outside the container
{
  admin 0.0.0.0:2019
}
```

But that exposes `/stop`, `/load`, `/config` (the full admin API) to the pod network. Almost never what you want. Use `exec` probes.

## The legacy `/healthz` endpoint

There's a `/healthz` endpoint on the main HTTP port. It only checks that the Caddy process is alive, not that the transport is healthy. **It is deprecated.** Don't add it to new probes; migrate existing probes to `/mercure/health/*`.

## Prometheus metrics

Enable metrics in `GLOBAL_OPTIONS`:

```caddyfile
# Prometheus metrics
{
  servers :443 {
    metrics
  }
}
```

Metrics live on the admin API at `/metrics`. The hub exposes Caddy's built-in metrics plus Mercure-specific ones:

| Metric                            | Description                              |
| --------------------------------- | ---------------------------------------- |
| `mercure_subscribers_connected`   | Current number of connected subscribers. |
| `mercure_subscribers_total`       | Total subscribers seen.                  |
| `mercure_updates_total`           | Total updates dispatched.                |
| `mercure_updates_failed_total`    | Updates that failed dispatch.            |
| `mercure_subscriber_list_cache_*` | Subscriber list cache stats.             |

Plus standard Caddy metrics: request counts, latencies, in-flight requests, certificate expiry. See the [Caddy metrics docs](https://caddyserver.com/docs/metrics).

## Useful alerts for the Mercure hub

Order matters, start with these, add more once you've learned your hub's normal behavior:

| Alert                    | Condition                                                                            |
| ------------------------ | ------------------------------------------------------------------------------------ |
| Hub down                 | `mercure_subscribers_connected` absent for >5 min on a pod that should have traffic. |
| Reconnect storm          | `rate(mercure_subscribers_total[5m])` > 10x steady state.                            |
| Transport unhealthy      | Readiness endpoint returning 503.                                                    |
| Update dispatch failures | `rate(mercure_updates_failed_total[5m]) / rate(mercure_updates_total[5m]) > 0.01`.   |
| Slow dispatch            | Caddy request duration p99 on the hub URL above your SLO.                            |
| Cert expiry              | Less than 14 days.                                                                   |

## Mercure grafana dashboards

A reasonable Grafana panel set:

- **Connections**: `mercure_subscribers_connected` per pod, stacked.
- **Publish rate**: `rate(mercure_updates_total[1m])`, with `mercure_updates_failed_total` overlaid.
- **Reconnect rate**: `rate(mercure_subscribers_total[1m])`. Spikes correlate with deploys, ingress restarts, and cert renewals.
- **Transport health**: readiness endpoint state (a synthetic probe writing to a metric).
- **Latency**: request duration histograms from Caddy.

## Application-level Mercure health canaries

A working hub does more than answer probes, it has to actually ferry data. A useful synthetic check that fully exercises the pipeline:

```bash
#!/bin/sh
# canary.sh, run from outside the cluster
JWT=$(generate-publisher-jwt)
TOPIC="https://example.com/_canary/$(date +%s)"

# Subscribe in the background
(curl -sN "https://hub.example.com/.well-known/mercure?match=$TOPIC" \
   --max-time 10 > /tmp/sub.txt) &
sleep 1

# Publish
curl -sX POST https://hub.example.com/.well-known/mercure \
  -H "Authorization: Bearer $JWT" \
  -d "topic=$TOPIC" -d "data=ping"

wait
grep -q "data: ping" /tmp/sub.txt || { echo "canary failed"; exit 1; }
echo "canary ok"
```

Run it from a different network than the hub (a CI runner, a separate cloud account). It catches problems that internal probes miss: ingress misconfigurations, certificate issues, CORS regressions.

## Mercure hub logging

The hub logs to stdout in JSON. Useful fields:

- `level`: `info`, `warn`, `error`.
- `msg`: human-readable description.
- `mercure.subscriber.id`: subscriber ID (when present).
- `caddy.error.*`: TLS, listener, transport errors.

For deeper diagnostics, set `GLOBAL_OPTIONS=debug`. **Don't leave it on in production:** it logs full update payloads, which means private data ends up in your log pipeline.

## Mercure hub performance baselines

What "normal" looks like, roughly:

- A subscriber costs ~10 KB of RAM and one goroutine.
- Per-publish CPU scales with the number of matching subscribers.
- Dispatch latency dominated by the slowest subscriber (`dispatch_timeout` caps it).
- Memory growth flat once subscriber count plateaus; a steady upward drift means a goroutine leak: file an issue with a `pprof` snapshot.

## Next steps for Mercure monitoring

- [Debugging](debugging.md): when metrics aren't enough.
- [Load testing](load-testing.md): establish a baseline before going live.
- [Rolling updates](rolling-updates.md): what your `mercure_subscribers_connected` chart should look like during a deploy.
