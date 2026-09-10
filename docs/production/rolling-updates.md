---
title: "Mercure rolling updates and graceful SSE shutdown"
description: "Drain Server-Sent Events connections cleanly during Mercure restarts and rolling updates with write_timeout, drain_timeout, and orchestrator grace periods."
---

# Rolling updates and graceful shutdown

SSE connections are long-lived by design. A naive restart (kill the process, start the new one) severs every active subscriber at the same instant. Each client auto-reconnects, all at the same moment, producing a sharp reconnect storm on the ingress and the transport: TLS handshakes, upstream renegotiation, transport churn. Visible to users as "the realtime UI freezes for a few seconds."

The Mercure.rocks Hub is built to avoid this. Shutdown rides the same `write_timeout` that already rotates connections in steady state, so a restart looks, from a client's perspective, like normal churn.

## How draining works

When the hub receives a shutdown signal (`SIGTERM`, the Caddy admin `/stop` endpoint, a graceful config reload), active subscriber handlers stay running. Each one exits when:

- the client disconnects, **or**
- the per-connection write deadline fires (derived from `write_timeout`, optionally shortened by JWT `exp`).

Because `write_timeout` already closes each SSE connection every few minutes during steady state and relies on the client to reconnect, letting shutdown ride the same timer spreads reconnects naturally over the drain window rather than triggering them all at once. No client-visible error, no storm: just the reconnect cadence browsers and SDKs already handle.

By default the drain window equals `write_timeout`. If that coupling forces an awkward trade-off between rare steady-state reconnects and fast rollouts, set `drain_timeout` to size the shutdown drain independently — see [Decoupling the drain window](#decoupling-the-drain-window-from-steady-state-rotation) below.

If `write_timeout` is `0s` (steady-state rotation disabled) and no `drain_timeout` is set, the hub exits all subscribers immediately on shutdown. At that point you've opted out of the drain mechanism, so the alternative would be to hang forever on active handlers. Setting `drain_timeout` restores a graceful drain even with `write_timeout 0s`.

## Sizing the drain window

The orchestrator must give the hub enough time between `SIGTERM` and `SIGKILL`. If it doesn't, the drain mechanism does nothing.

**The rule:** `stop timeout >= write_timeout + small margin`.

For the default `write_timeout 600s`, a 660s grace period is the right starting point. If you bump `write_timeout`, bump the orchestrator's grace period to match — unless you set `drain_timeout` (see below), which caps the drain window independently.

## Decoupling the drain window from steady-state rotation

`write_timeout` does double duty: it sets how often each connection rotates in steady state **and** how long a shutdown takes to drain. Raising it to cut steady-state reconnects therefore also stretches every rolling update. A four-pod, `maxUnavailable: 0` rollout at `write_timeout 1200s` can take over an hour, because each pod waits up to 20 minutes to drain before the next one rotates.

`drain_timeout` breaks that coupling. When set, a termination reschedules each subscriber to close at a random point spread uniformly across the `drain_timeout` window, instead of riding its (longer) `write_timeout` deadline. Reconnects still spread out — no storm — but the whole drain finishes within `drain_timeout` no matter how long `write_timeout` is.

```caddyfile
localhost {
  mercure {
    write_timeout 1200s  # rare steady-state reconnects (~20 min rotation)
    drain_timeout 300s   # but drain within 5 minutes on shutdown
  }
}
```

With `drain_timeout` set, the grace-period rule keys on the drain window instead of `write_timeout`:

**The rule (with `drain_timeout`):** `stop timeout >= drain_timeout + small margin`.

So `write_timeout 1200s` + `drain_timeout 300s` needs only a ~360s grace period, and each pod's drain paces over 5 minutes rather than 20.

Two details worth knowing:

- **`write_timeout 0s` + `drain_timeout`.** Steady-state rotation is off (connections never rotate on their own), but a termination still drains them gracefully over the window instead of cutting them all at once.
- **Reloads never drain.** A graceful config reload (`caddy reload`, `SIGUSR1`) is not a termination, so it never triggers the drain regardless of `drain_timeout`: a connection that has a write deadline of its own (`write_timeout` above `0s`, or a JWT `exp`) stays reconnect-free, riding that deadline across the swap. The drain applies only to real termination — `SIGTERM`, `SIGINT`, and the Caddy admin `/stop` endpoint. The exception is `write_timeout 0s`: with no deadline of its own, such a connection is closed on reload through the escape hatch that keeps shutdown from hanging on idle handlers, and `drain_timeout` — termination-only — doesn't spare it.

## Kubernetes

The Helm chart ships with SSE-appropriate defaults:

- `terminationGracePeriodSeconds: 660`: matches the 600s default `write_timeout` plus 60s margin.
- `strategy.rollingUpdate.maxSurge: 1, maxUnavailable: 0`: one pod rotates at a time, no capacity drop.
- `minReadySeconds: 30`: a newly-Ready pod gets time to warm its transport before the next rotation.

A four-pod rolling update with these settings turns into a reconnect stream paced by `write_timeout`, spread over tens of minutes, instead of a four-wave storm hitting the ingress in a few seconds.

If you set `write_timeout` higher than 600s, either raise `terminationGracePeriodSeconds` proportionally:

```yaml
# Kubernetes
terminationGracePeriodSeconds: 960 # for write_timeout 900s
```

or set a shorter `drain_timeout` and size the grace period to that instead (see [Decoupling the drain window](#decoupling-the-drain-window-from-steady-state-rotation)), which keeps rollouts fast without lowering `write_timeout`.

If the grace period covers neither, `kubelet` `SIGKILL`s the pod mid-drain and the storm you were avoiding lands anyway.

## Why `minReadySeconds` matters for Mercure rollouts

Once Kubernetes marks a new pod Ready, the transport inside it still needs a moment to reach steady state: open the BoltDB cursor, start the Redis `XREAD` loop, join the Kafka consumer group. Without `minReadySeconds`, Kubernetes rotates the next pod as soon as the readiness probe passes, which fires before the backend is fully online.

With 30s of quiet time, each pod stabilizes before taking its share of load. The chart sets this by default; don't lower it without measuring.

## Non-Kubernetes deployments

Any supervisor that gives the hub time to drain works the same way:

| Supervisor | Equivalent          |
| ---------- | ------------------- |
| systemd    | `TimeoutStopSec`    |
| Docker     | `--stop-timeout`    |
| Compose    | `stop_grace_period` |
| Nomad      | `kill_timeout`      |
| ECS        | `stopTimeout`       |

The rule is the same: stop timeout >= the drain window + small margin — that's `write_timeout`, or `drain_timeout` when you've set it.

## Graceful Mercure hub configuration reloads

`caddy reload` (or sending `SIGUSR1`) reloads the config without dropping active connections; the listener is shared across processes during the swap. SSE connections flow uninterrupted.

This is the cleanest way to roll a config change in production: zero reconnects, zero downtime — the shared listener carries each connection across the swap, riding its existing write deadline. A reload is not a termination, so `drain_timeout` never applies to it. The exception is `write_timeout 0s`: such a connection has no deadline of its own and is closed on reload through the escape hatch (an immediate reconnect), which `drain_timeout` — being termination-only — does not prevent.

## Self-hosted transports

The drain mechanism is built into the open-source hub and works with BoltDB. The [Self-Hosted transports](high-availability.md) (Redis, PostgreSQL, Kafka, Pulsar) inherit it automatically: each connection drains on its own timer (`write_timeout`, or `drain_timeout` on shutdown) regardless of which backend carries the updates.

For deployments that can't afford any restart-related reconnect (sub-second SLOs, strict steady-state requirements), [Cloud and Self-Hosted](https://mercure.rocks/pricing) additionally run multi-node clusters that route around individual replica restarts entirely. A single replica restarting doesn't reconnect any clients at all, because they're balanced across the others.

## Verifying the drain

Watch the active subscribers metric (`mercure_subscribers_connected`) during a deploy. Healthy drains look like a smooth ramp down on the old replica and a matching ramp up on the new one. A cliff to zero is a misconfigured grace period.

A quick sanity check from the command line:

```console
# Verifying the drain
kubectl exec -it $POD -- wget -qO- localhost:2019/metrics | grep subscribers_connected
```

Trigger a `kubectl rollout restart deployment/mercure` and watch the value glide rather than collapse.

## What Mercure clients see during a rolling update

Either nothing (the steady-state rotation already does this every `write_timeout`) or a single reconnect with `Last-Event-ID` set. The hub's history covers the brief gap; clients pick up where they left off. Browsers and the major SSE libraries handle this without prompting.

## Next steps for Mercure rolling updates

- [Configuration](../deployment/configuration.md): `write_timeout` and friends.
- [High availability](high-availability.md): when even smooth restarts aren't enough.
- [Health monitoring](health-monitoring.md): verifying the new pod is actually serving before draining the old one.
