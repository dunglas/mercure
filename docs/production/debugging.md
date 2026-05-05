---
title: "Debugging the Mercure.rocks Hub with pprof"
description: "Profile the Mercure.rocks Hub with pprof, capture heap and goroutine snapshots, and trace request latency and lock contention."
---

# Debugging

When metrics tell you something is wrong but not what, reach for the profiler. The hub ships [`pprof`](https://blog.golang.org/pprof) — Go's CPU, heap, goroutine, and lock profiler.

## Enable the Mercure Hub pprof Profiler

Add `debug` to the Caddyfile's global options:

```caddyfile
# Enable the Mercure Hub pprof Profiler
{
  debug
}

# ...

```

Or set the environment variable:

```console
# Enable the Mercure Hub pprof Profiler
GLOBAL_OPTIONS=debug
```

The profiler exposes its endpoints at `http://localhost:2019/debug/pprof/` (the Caddy admin port). Like the health endpoints, this binds to localhost — reach it via `kubectl exec` or `docker exec`.

> **Production safety.** `debug` mode also makes Caddy log more verbosely, including update payloads when present in errors. The profiler endpoints themselves don't expose data, only profiles. For an always-on production deployment, prefer toggling `debug` only when you need to investigate something.

## What's available

Visit `http://localhost:2019/debug/pprof/` for the full list. The ones that matter most:

| Profile | Use it for |
| --- | --- |
| `heap` | Memory leaks, oversized allocations. |
| `goroutine` | Goroutine leaks (the hub keeping handlers alive after disconnect). |
| `profile?seconds=30` | CPU profile over a window. Find hot paths. |
| `block` | Goroutines blocked on synchronization. |
| `mutex` | Mutex contention. |
| `allocs` | Past allocations (cumulative since start). |

## Capture a CPU Profile of the Mercure Hub

Capture a 30-second CPU profile and view it in the browser:

```console
# Capture a CPU Profile of the Mercure Hub
go tool pprof -http=:8080 http://localhost:2019/debug/pprof/profile?seconds=30
```

While `pprof` is sampling, drive load against the hub. The flame graph will show where time is spent — usually in matcher evaluation, dispatch, or transport I/O for a healthy hub.

## Capture a Heap Snapshot of the Mercure Hub

```console
# Capture a Heap Snapshot of the Mercure Hub
go tool pprof -http=:8080 http://localhost:2019/debug/pprof/heap
```

The `inuse_space` view shows current live memory. The `alloc_space` view shows cumulative — useful for finding allocation hotspots that aren't necessarily leaks.

If memory grows monotonically during steady state, that's a leak. Capture two heap snapshots a few minutes apart and diff:

```console
# Capture a Heap Snapshot of the Mercure Hub
curl -s http://localhost:2019/debug/pprof/heap > heap1.pb.gz
sleep 300
curl -s http://localhost:2019/debug/pprof/heap > heap2.pb.gz
go tool pprof -http=:8080 -base heap1.pb.gz heap2.pb.gz
```

## Capture a Goroutine Dump of the Mercure Hub

A goroutine dump is the cheapest way to diagnose "the hub is wedged":

```console
# Capture a Goroutine Dump of the Mercure Hub
curl -s http://localhost:2019/debug/pprof/goroutine?debug=2 > goroutines.txt
```

Look for:

- Goroutines stuck in transport reads (Redis `XREAD`, Postgres `LISTEN`) — usually fine, expected behavior.
- Goroutines stuck in `chan send` — backpressure on the dispatch path. A slow subscriber blocking everyone.
- Goroutines piling up on the same handler over time — leaked subscriber handlers; usually a missed `defer`.

## Capture an Execution Trace of the Mercure Hub

For latency investigations, capture an execution trace:

```console
# Capture an Execution Trace of the Mercure Hub
curl -s "http://localhost:2019/debug/pprof/trace?seconds=10" -o trace.out
go tool trace trace.out
```

Trace lets you see scheduler decisions, GC pauses, and per-goroutine timing. Use it when you need to understand *when* something happened, not just *what*.

## Past Allocations Profile for the Mercure Hub

```console
# Past Allocations Profile for the Mercure Hub
go tool pprof -http=:8080 http://localhost:2019/debug/pprof/allocs
```

Useful when chasing GC pressure: which call sites are allocating the most over time.

## What healthy looks like

For a hub serving 10k subscribers, roughly:

- ~40% of CPU in epoll/network reads, idle most of the time.
- ~5–15% in dispatch fan-out under steady publish load.
- A handful of long-lived goroutines per subscriber.
- Stable RSS after the connection count plateaus.

Anomalies show up against this baseline. Capture a profile from a healthy hub and keep it as a reference.

## When to Escalate a Mercure Hub Performance Issue

The Mercure team helps debug performance issues for [Cloud and Self-Hosted](https://mercure.rocks/pricing) customers, with priority response on Business and Corporate tiers. For the open-source hub, file a GitHub issue with:

- A heap or goroutine snapshot demonstrating the issue.
- Hub version (`./mercure version`).
- Caddyfile (with secrets redacted).
- Observed metric anomaly with a graph if you have one.

Reproducible cases are fixed faster.

## Next Steps for Mercure Debugging

- [Health monitoring](health-monitoring.md) — what to watch normally.
- [Load testing](load-testing.md) — establish a baseline before chasing regressions.
