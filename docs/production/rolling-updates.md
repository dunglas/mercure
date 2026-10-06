---
title: "Mercure rolling updates and graceful SSE shutdown"
description: "Drain Server-Sent Events connections cleanly during Mercure restarts and rolling updates with write_timeout and orchestrator grace periods."
---

# Mercure rolling updates and graceful shutdown

Restarting a hub disconnects its SSE subscribers. If all connections close together, clients can overload the new instance with simultaneous reconnections.

The Mercure.rocks Hub lets connections drain until their existing write deadlines. With a shared transport and another ready replica, clients can reconnect while the old instance shuts down.

## How draining works

When the hub receives a shutdown signal (`SIGTERM`, the Caddy admin `/stop` endpoint, a graceful config reload), active subscriber handlers stay running. Each one exits when:

- the client disconnects, **or**
- the per-connection write deadline fires (derived from `write_timeout`, optionally shortened by JWT `exp`).

The hub randomizes connection deadlines between 80% and 100% of `write_timeout` (480 to 600 seconds by default). During shutdown, existing connections use those deadlines. This spreads reconnections, although the distribution still depends on when clients connected.

With `write_timeout 0s`, the hub closes subscribers immediately on shutdown.

## Sizing the drain window

The orchestrator must allow enough time between `SIGTERM` and `SIGKILL`; otherwise, remaining connections are terminated before they finish draining.

**The rule:** `stop timeout >= write_timeout + small margin`.

For the default `write_timeout 600s`, a 660s grace period is the right starting point. If you bump `write_timeout`, bump the orchestrator's grace period to match.

## Kubernetes

For `RollingUpdate`, the Helm chart sets:

- `terminationGracePeriodSeconds: 660`: matches the 600s default `write_timeout` plus 60s margin.
- `strategy.rollingUpdate.maxSurge: 1, maxUnavailable: 0`: one pod rotates at a time, no capacity drop.
- `minReadySeconds: 30`: a newly-Ready pod gets time to warm its transport before the next rotation.

Rolling updates can take several minutes. Size the deployment progress deadline as well as the pod termination grace period.

If you set `write_timeout` higher than 600s, raise `terminationGracePeriodSeconds` proportionally:

```yaml
terminationGracePeriodSeconds: 960 # for write_timeout 900s
```

A shorter grace period forces the remaining subscribers to disconnect together.

## Why `minReadySeconds` matters for Mercure rollouts

`minReadySeconds` requires a new pod to remain ready before Kubernetes treats it as available. It helps pace a rollout and reveal failures soon after startup; it does not delay traffic to an already-ready pod.

The chart uses 30 seconds for rolling updates. Readiness must still reflect whether the transport can serve traffic.

## Non-Kubernetes deployments

Any supervisor that gives the hub time to drain works the same way:

| Supervisor | Equivalent          |
| ---------- | ------------------- |
| systemd    | `TimeoutStopSec`    |
| Docker     | `--stop-timeout`    |
| Compose    | `stop_grace_period` |
| Nomad      | `kill_timeout`      |
| ECS        | `stopTimeout`       |

Choose `write_timeout` below the supervisor's maximum stop timeout, with margin. Some platforms cap that timeout below the hub's default 600 seconds.

## Graceful Mercure hub configuration reloads

Use `caddy reload --config /etc/caddy/Caddyfile` to apply changes without restarting the process. Caddy can reuse listeners and unchanged transports. Changes that replace a hub or transport may cause subscriptions to drain and reconnect, so test reloads with your configuration.

On supported systems, `SIGUSR1` can reload the startup configuration file when Caddy was started from one and its signal-reload conditions are met. See [Caddy signals](https://caddyserver.com/docs/command-line#signals).

## Self-hosted transports

The drain mechanism is built into the open-source hub and works with BoltDB. The [Self-Hosted transports](high-availability.md) (Redis/Valkey, PostgreSQL, Kafka, Pulsar) inherit it automatically: each connection drains at its own `write_timeout` regardless of which backend carries the updates.

[Choose an Enterprise plan](https://mercure.rocks/pricing) for shared transports and maintainer support, or let [Mercure Cloud](https://mercure.rocks/pricing) handle the rollout. A shared transport lets a reconnecting client resume on another replica. Clients attached to a restarting replica still reconnect; a load balancer cannot migrate an established SSE connection.

## Verifying the drain

Watch the active subscribers metric (`mercure_subscribers_connected`) during a deploy. Healthy drains look like a smooth ramp down on the old replica and a matching ramp up on the new one. A sudden drop may indicate forced termination; compare it with the remaining connection count and shutdown logs.

A quick sanity check from the command line:

```console
kubectl exec -it $POD -- wget -qO- localhost:2019/metrics | grep subscribers_connected
```

Trigger a `kubectl rollout restart deployment/mercure` and watch the value glide rather than collapse.

## What Mercure clients see during a rolling update

Clients reconnect after their stream closes. `EventSource` sends the last event ID it received, and the hub replays retained matching updates. If the history no longer covers the gap, the application must resynchronize.

## Next steps

- [Configuration](../deployment/configuration.md): timeout settings.
- [High availability](high-availability.md): shared transports and redundant nodes.
- [Health monitoring](health-monitoring.md): checking new replicas before retiring old ones.
