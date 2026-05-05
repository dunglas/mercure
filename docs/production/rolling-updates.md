# Rolling Updates and Graceful Shutdown

SSE connections are long-lived by design. A naive restart — kill the process, start the new one — severs every active subscriber at the same instant. Each client auto-reconnects, all at the same moment, producing a sharp reconnect storm on the ingress and the transport: TLS handshakes, upstream renegotiation, transport churn. Visible to users as "the realtime UI freezes for a few seconds."

The Mercure.rocks Hub is built to avoid this. Shutdown rides the same `write_timeout` that already rotates connections in steady state, so a restart looks, from a client's perspective, like normal churn.

## How draining works

When the hub receives a shutdown signal (`SIGTERM`, the Caddy admin `/stop` endpoint, a graceful config reload), active subscriber handlers stay running. Each one exits when:

- the client disconnects, **or**
- the per-connection write deadline fires (derived from `write_timeout`, optionally shortened by JWT `exp`).

Because `write_timeout` already closes each SSE connection every few minutes during steady state and relies on the client to reconnect, letting shutdown ride the same timer spreads reconnects naturally over the drain window rather than triggering them all at once. No client-visible error, no storm — just the reconnect cadence browsers and SDKs already handle.

If `write_timeout` is `0s` (steady-state rotation disabled), the hub exits all subscribers immediately on shutdown. At that point you've opted out of the drain mechanism, so the alternative would be to hang forever on active handlers.

## Sizing the drain window

The orchestrator must give the hub enough time between `SIGTERM` and `SIGKILL`. If it doesn't, the drain mechanism does nothing.

**The rule:** `stop timeout ≥ write_timeout + small margin`.

For the default `write_timeout 600s`, a 660s grace period is the right starting point. If you bump `write_timeout`, bump the orchestrator's grace period to match.

## Kubernetes

The Helm chart ships with SSE-appropriate defaults:

- `terminationGracePeriodSeconds: 660` — matches the 600s default `write_timeout` plus 60s margin.
- `strategy.rollingUpdate.maxSurge: 1, maxUnavailable: 0` — one pod rotates at a time, no capacity drop.
- `minReadySeconds: 30` — a newly-Ready pod gets time to warm its transport before the next rotation.

A four-pod rolling update with these settings turns into a reconnect stream paced by `write_timeout`, spread over tens of minutes — instead of a four-wave storm hitting the ingress in a few seconds.

If you set `write_timeout` higher than 600s, raise `terminationGracePeriodSeconds` proportionally:

```yaml
terminationGracePeriodSeconds: 960  # for write_timeout 900s
```

If you don't, `kubelet` `SIGKILL`s the pod mid-drain and the storm you were avoiding lands anyway.

## `minReadySeconds` matters

Once Kubernetes marks a new pod Ready, the transport inside it still needs a moment to reach steady state — open the BoltDB cursor, start the Redis `XREAD` loop, join the Kafka consumer group. Without `minReadySeconds`, Kubernetes rotates the next pod as soon as the readiness probe passes, which fires before the backend is fully online.

With 30s of quiet time, each pod stabilizes before taking its share of load. The chart sets this by default; don't lower it without measuring.

## Non-Kubernetes deployments

Any supervisor that gives the hub time to drain works the same way:

| Supervisor | Equivalent |
| --- | --- |
| systemd | `TimeoutStopSec` |
| Docker | `--stop-timeout` |
| Compose | `stop_grace_period` |
| Nomad | `kill_timeout` |
| ECS | `stopTimeout` |

The rule is the same: stop timeout ≥ `write_timeout` + small margin.

## Graceful config reloads

`caddy reload` (or sending `SIGUSR1`) reloads the config without dropping active connections — the listener is shared across processes during the swap. SSE connections flow uninterrupted.

This is the cleanest way to roll a config change in production: zero reconnects, zero downtime, regardless of `write_timeout`.

## Self-Hosted transports

The drain mechanism is built into the open-source hub and works with BoltDB. The [Self-Hosted transports](high-availability.md) (Redis, PostgreSQL, Kafka, Pulsar) inherit it automatically — each connection drains at its own `write_timeout` regardless of which backend carries the updates.

For deployments that can't afford any restart-related reconnect (sub-second SLOs, strict steady-state requirements), [Cloud and Self-Hosted](https://mercure.rocks/pricing) additionally run multi-node clusters that route around individual replica restarts entirely — a single replica restarting doesn't reconnect any clients at all, because they're balanced across the others.

## Verifying the drain

Watch the active subscribers metric (`mercure_subscribers_connected`) during a deploy. Healthy drains look like a smooth ramp down on the old replica and a matching ramp up on the new one. A cliff to zero is a misconfigured grace period.

A quick sanity check from the command line:

```console
kubectl exec -it $POD -- wget -qO- localhost:2019/metrics | grep subscribers_connected
```

Trigger a `kubectl rollout restart deployment/mercure` and watch the value glide rather than collapse.

## What clients see

Either nothing (the steady-state rotation already does this every `write_timeout`) or a single reconnect with `Last-Event-ID` set. The hub's history covers the brief gap; clients pick up where they left off. Browsers and the major SSE libraries handle this without prompting.

## Next

- [Configuration](../deployment/configuration.md) — `write_timeout` and friends.
- [High availability](high-availability.md) — when even smooth restarts aren't enough.
- [Health monitoring](health-monitoring.md) — verifying the new pod is actually serving before draining the old one.
