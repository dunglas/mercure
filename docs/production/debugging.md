---
title: "Debugging the Mercure.rocks hub with pprof"
description: "Profile the Mercure.rocks Hub with pprof, capture heap and goroutine snapshots, and trace request latency and lock contention."
---

# Mercure debugging

Use the Go `pprof` endpoints to investigate CPU, memory, and goroutine use. Compare profiles from representative workloads before diagnosing a regression.

## Enable the Mercure hub pprof profiler

Caddy exposes `pprof` at `http://localhost:2019/debug/pprof/` while its admin API is enabled. The `debug` logging option is not required. Reach the loopback endpoint through a local command, `docker exec`, or `kubectl exec`.

Keep the admin API private. Profiles and goroutine dumps can disclose application details, and the admin API also permits configuration changes.

## What's available

Visit `http://localhost:2019/debug/pprof/` for the full list. The ones that matter most:

| Profile              | Use it for                                                         |
| -------------------- | ------------------------------------------------------------------ |
| `heap`               | Memory leaks, oversized allocations.                               |
| `goroutine`          | Goroutine leaks (the hub keeping handlers alive after disconnect). |
| `goroutineleak`      | Only the goroutines the runtime proved are permanently blocked.    |
| `profile?seconds=30` | CPU profile over a window. Find hot paths.                         |
| `block`              | Goroutines blocked on synchronization.                             |
| `mutex`              | Mutex contention.                                                  |
| `allocs`             | Past allocations (cumulative since start).                         |

## Capture a CPU profile of the Mercure hub

Capture a 30-second CPU profile and view it in the browser:

```console
go tool pprof -http=:8080 "http://localhost:2019/debug/pprof/profile?seconds=30"
```

While `pprof` is sampling, drive load against the hub. The flame graph will show where time is spent, usually in matcher evaluation, dispatch, or transport I/O for a healthy hub.

## Capture a heap snapshot of the Mercure hub

```console
go tool pprof -http=:8080 http://localhost:2019/debug/pprof/heap
```

The `inuse_space` view shows current live memory. The `alloc_space` view shows cumulative, useful for finding allocation hotspots that aren't necessarily leaks.

Sustained memory growth under comparable load can indicate a leak or retained data. Capture two heap snapshots and compare them:

```console
curl -s http://localhost:2019/debug/pprof/heap > heap1.pb.gz
sleep 300
curl -s http://localhost:2019/debug/pprof/heap > heap2.pb.gz
go tool pprof -http=:8080 -base heap1.pb.gz heap2.pb.gz
```

## Capture a goroutine dump of the Mercure hub

A goroutine dump is the cheapest way to diagnose "the hub is wedged":

```console
curl -s "http://localhost:2019/debug/pprof/goroutine?debug=2" > goroutines.txt
```

Look for:

- Goroutines stuck in transport reads (Redis `XREAD`, Postgres `LISTEN`): usually fine, expected behavior.
- Goroutines stuck in `chan send`: backpressure on the dispatch path. Check subscriber queues and dispatch timeouts.
- Goroutines piling up on the same handler over time: leaked subscriber handlers; usually a missed `defer`.

## Find leaked goroutines in the Mercure hub

The `goroutine` profile lists every live goroutine, so a leak hides among the
per-subscriber goroutines a healthy hub is supposed to have. The `goroutineleak`
profile reports only goroutines the garbage collector proved can never be
unblocked, which is the shorter list worth reading:

```console
curl -s "http://localhost:2019/debug/pprof/goroutineleak?debug=2" > leaks.txt
```

An empty profile means no leak was detected. Goroutines blocked on a channel or
mutex still reachable from a global variable can escape detection, so an empty
result is not a proof of absence: fall back to the goroutine dump above.

## Capture an execution trace of the Mercure hub

For latency investigations, capture an execution trace:

```console
curl -s "http://localhost:2019/debug/pprof/trace?seconds=10" -o trace.out
go tool trace trace.out
```

Trace lets you see scheduler decisions, GC pauses, and per-goroutine timing. Use it when you need to understand _when_ something happened, not just _what_.

## Past allocations profile for the Mercure hub

```console
go tool pprof -http=:8080 http://localhost:2019/debug/pprof/allocs
```

Useful when chasing GC pressure: which call sites are allocating the most over time.

## What healthy looks like

Record a baseline at your expected connection count, publish rate, payload size, and transport configuration. Compare CPU profiles, live heap, and goroutine counts under the same load. There is no fixed CPU percentage or memory cost per subscriber that applies to every deployment.

## When to escalate a Mercure hub performance issue

For commercial support, see [Mercure plans](https://mercure.rocks/pricing). For community support, open an issue with a reproducible case and:

- A heap or goroutine snapshot demonstrating the issue.
- Hub version (`./mercure version`).
- Caddyfile (with secrets redacted).
- Observed metric anomaly with a graph if you have one.

Redact secrets and review profiles for sensitive data before sharing them.

## Next steps

- [Health monitoring](health-monitoring.md): what to watch normally.
- [Load testing](load-testing.md): establish a baseline before chasing regressions.
