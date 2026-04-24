# Rolling Updates and Graceful Shutdown

Server-Sent Events connections are long-lived by design. A naive restart of a hub replica — just killing the process — severs every active subscription at the same instant. Every client auto-reconnects, but all at the same moment, producing a sharp reconnect storm on the ingress and on the transport backend: TLS handshakes, upstream renegotiation, transport churn.

The Mercure.rocks Hub is built to avoid this. Shutdown is interleaved with the `write_timeout` that already rotates each client connection during steady-state operation, so restarts and config reloads look, from a client's perspective, like regular churn. This page explains the mechanism and how to configure your orchestrator to take advantage of it.

## How the Hub Drains on Shutdown

When the Hub receives a shutdown signal — the Caddy admin `/stop` endpoint, `SIGTERM`, a graceful config reload — the active subscriber handlers stay running. They exit only when:

- the client disconnects, **or**
- the per-connection write deadline fires (derived from `write_timeout`, optionally shortened by JWT `exp`).

Because `write_timeout` already closes each SSE connection every few minutes in steady state and relies on the client to reconnect, letting shutdown ride the same timer spreads reconnects naturally over the drain window rather than triggering them all at once. No client-visible error, no storm — just the usual SSE retry cadence browsers and SDKs are already handling.

When `write_timeout` is set to `0s`, there is no per-connection timer; in that case, the Hub does exit all subscribers immediately on shutdown, since the alternative would be to hang forever on active handlers.

## Kubernetes

For the drain to actually happen, Kubernetes must give the Hub enough time between `SIGTERM` and `SIGKILL`. That's the Pod spec's `terminationGracePeriodSeconds`. It must be at least `write_timeout` plus a small margin.

The Helm chart ships with SSE-appropriate defaults out of the box:

- `terminationGracePeriodSeconds: 660` — matches the 600s `write_timeout` default plus a 60s margin.
- `strategy.rollingUpdate.maxSurge: 1`, `maxUnavailable: 0` — one pod rotates at a time, without dropping capacity.
- `minReadySeconds: 30` — a newly-Ready pod gets 30s to reach transport steady state before the next rotation begins.

Together, they turn a rolling update of a four-pod Hub into a reconnect stream paced by `write_timeout`, spread over tens of minutes, rather than a four-wave storm hitting the ingress in a few seconds.

If you override `write_timeout` (via the `write_timeout` Caddyfile directive), bump `terminationGracePeriodSeconds` to at least the new value plus a margin — otherwise Kubernetes will `SIGKILL` the pod mid-drain and the storm you were trying to avoid lands anyway.

### Why `minReadySeconds`?

Once Kubernetes marks a new pod as Ready, the transport backend inside it still needs a moment to reach steady state: warming the BoltDB cursor, opening the Redis `XREAD` loop, joining a Kafka consumer group, etc. Without `minReadySeconds`, Kubernetes would rotate the next pod as soon as the readiness probe passes — which fires before the backend has caught up. With 30s of quiet time, each pod has a chance to stabilize before taking its share of load.

## Non-Kubernetes Deployments

Any supervisor that gives the Hub time to drain works the same way. systemd's `TimeoutStopSec`, Docker's `--stop-timeout`, and Nomad's `kill_timeout` all play the role of `terminationGracePeriodSeconds`.

Whatever you use, the rule is the same: **stop timeout ≥ `write_timeout` + small margin**. Anything lower and your graceful shutdown isn't graceful — it's a delayed `SIGKILL`.

## The Enterprise Transports

The drain mechanism described here is built into the open source Hub and works with the BoltDB transport. [The Enterprise transports](cluster.md) (Redis, PostgreSQL, Apache Kafka, Apache Pulsar) inherit it automatically: each connection drains at its own `write_timeout` regardless of which backend carries the updates.

For deployments that cannot afford any restart-related reconnect — sub-second delivery SLOs, strict steady-state requirements — [the Cloud and Enterprise versions](cluster.md) additionally run multi-node clusters that route around individual replica restarts entirely.
